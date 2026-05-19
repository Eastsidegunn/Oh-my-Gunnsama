package daemon

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"oh-my-gunnsama/internal/protocol"
)

var ErrAlreadyRunning = errors.New("omg daemon already running")

type Handler func(context.Context, *protocol.Request) protocol.Response

type TimeoutTiers struct {
	OnSubmit     time.Duration
	OnToolBefore time.Duration
	OnToolAfter  time.Duration
	OnError      time.Duration
	OnCompact    time.Duration
	OnStop       time.Duration
}

type Options struct {
	SocketPath string
	LockPath   string
	Timeouts   TimeoutTiers
	Handler    Handler
}

type Server struct {
	options  Options
	listener net.Listener
	lockFile *os.File
	closed   chan struct{}
	once     sync.Once
	mu       sync.Mutex
}

func NewServer(options Options) *Server {
	options.Timeouts = withDefaultTimeouts(options.Timeouts)
	if options.Handler == nil {
		options.Handler = func(_ context.Context, req *protocol.Request) protocol.Response {
			return protocol.OKResponse(req, protocol.ActionNone, "", nil)
		}
	}
	return &Server{options: options, closed: make(chan struct{})}
}

func (s *Server) Start(ctx context.Context) error {
	if s.options.SocketPath == "" {
		return fmt.Errorf("socket path is required")
	}
	if s.options.LockPath == "" {
		return fmt.Errorf("lock path is required")
	}

	if err := os.MkdirAll(filepath.Dir(s.options.SocketPath), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.options.LockPath), 0o755); err != nil {
		return err
	}

	lockFile, err := os.OpenFile(s.options.LockPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return ErrAlreadyRunning
		}
		return err
	}
	if _, err := fmt.Fprintf(lockFile, "%d\n", os.Getpid()); err != nil {
		_ = lockFile.Close()
		_ = os.Remove(s.options.LockPath)
		return err
	}

	if err := os.Remove(s.options.SocketPath); err != nil && !os.IsNotExist(err) {
		_ = lockFile.Close()
		_ = os.Remove(s.options.LockPath)
		return err
	}

	listener, err := net.Listen("unix", s.options.SocketPath)
	if err != nil {
		_ = lockFile.Close()
		_ = os.Remove(s.options.LockPath)
		return err
	}

	s.mu.Lock()
	s.lockFile = lockFile
	s.listener = listener
	s.mu.Unlock()

	go s.serve(ctx)
	return nil
}

func (s *Server) SocketPath() string {
	return s.options.SocketPath
}

func (s *Server) Close() error {
	var closeErr error
	s.once.Do(func() {
		s.mu.Lock()
		listener := s.listener
		lockFile := s.lockFile
		s.mu.Unlock()

		if listener != nil {
			closeErr = listener.Close()
		}
		if lockFile != nil {
			if err := lockFile.Close(); closeErr == nil {
				closeErr = err
			}
		}
		if err := os.Remove(s.options.SocketPath); closeErr == nil && err != nil && !os.IsNotExist(err) {
			closeErr = err
		}
		if err := os.Remove(s.options.LockPath); closeErr == nil && err != nil && !os.IsNotExist(err) {
			closeErr = err
		}
		close(s.closed)
	})
	return closeErr
}

func (s *Server) Done() <-chan struct{} {
	return s.closed
}

func (s *Server) serve(ctx context.Context) {
	go func() {
		<-ctx.Done()
		_ = s.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.closed:
				return
			default:
				_ = s.Close()
				return
			}
		}
		go s.handleConnection(ctx, conn)
	}
}

func (s *Server) handleConnection(parent context.Context, conn net.Conn) {
	defer conn.Close()

	req, perr := protocol.ReadRequest(conn)
	if perr != nil {
		_ = protocol.WriteResponse(conn, protocol.ErrorResponse(req, protocol.ActionWarn, perr, nil))
		return
	}

	requestTimeout := s.timeoutFor(req)
	ctx, cancel := context.WithTimeout(parent, requestTimeout)
	defer cancel()

	result := make(chan protocol.Response, 1)
	go func() {
		result <- s.options.Handler(ctx, req)
	}()

	select {
	case resp := <-result:
		_ = protocol.WriteResponse(conn, resp)
	case <-ctx.Done():
		perr := protocol.NewProtocolError(protocol.ErrorTimeout, "daemon request timed out", true)
		_ = protocol.WriteResponse(conn, protocol.ErrorResponse(req, protocol.ActionWarn, perr, nil))
	}
}

func (s *Server) timeoutFor(req *protocol.Request) time.Duration {
	if req != nil && req.TimeoutMS > 0 {
		requested := time.Duration(req.TimeoutMS) * time.Millisecond
		if requested < s.options.Timeouts.forEvent(req.Event) {
			return requested
		}
	}
	if req == nil {
		return s.options.Timeouts.OnError
	}
	return s.options.Timeouts.forEvent(req.Event)
}

func ContextWithSignals(parent context.Context) (context.Context, context.CancelFunc) {
	return signal.NotifyContext(parent, syscall.SIGINT, syscall.SIGTERM)
}

func (t TimeoutTiers) forEvent(event protocol.Event) time.Duration {
	switch event {
	case protocol.EventOnSubmit:
		return t.OnSubmit
	case protocol.EventOnToolBefore:
		return t.OnToolBefore
	case protocol.EventOnToolAfter:
		return t.OnToolAfter
	case protocol.EventOnCompact:
		return t.OnCompact
	case protocol.EventOnStop:
		return t.OnStop
	case protocol.EventOnError:
		fallthrough
	default:
		return t.OnError
	}
}

func withDefaultTimeouts(t TimeoutTiers) TimeoutTiers {
	if t.OnSubmit == 0 {
		t.OnSubmit = 500 * time.Millisecond
	}
	if t.OnToolBefore == 0 {
		t.OnToolBefore = 200 * time.Millisecond
	}
	if t.OnToolAfter == 0 {
		t.OnToolAfter = 200 * time.Millisecond
	}
	if t.OnError == 0 {
		t.OnError = 200 * time.Millisecond
	}
	if t.OnCompact == 0 {
		t.OnCompact = 500 * time.Millisecond
	}
	if t.OnStop == 0 {
		t.OnStop = 200 * time.Millisecond
	}
	return t
}
