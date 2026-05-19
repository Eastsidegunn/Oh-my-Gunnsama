package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oh-my-gunnsama/adapters/pi"
	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/daemon"
	"oh-my-gunnsama/internal/orchestrator"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/registry"
	"oh-my-gunnsama/internal/route"
	"oh-my-gunnsama/internal/storage"
)

type Client interface {
	RoundTrip(context.Context, protocol.Request) (protocol.Response, error)
}

type serverStarter interface {
	Start(context.Context, string, string, func(context.Context, *protocol.Request) protocol.Response) error
}

type managedStorageSink interface {
	storage.Sink
	Shutdown(context.Context) error
}

type dependencies struct {
	cwd           string
	home          string
	client        Client
	starter       serverStarter
	newSink       func(context.Context, string, storage.Options) (managedStorageSink, error)
	workerFactory orchestrator.WorkerFactory
	stderrDiag    bool
}

type unixClient struct {
	socketPath string
	timeout    time.Duration
}

type daemonStarter struct{}

func (d daemonStarter) Start(ctx context.Context, socketPath, lockPath string, handler func(context.Context, *protocol.Request) protocol.Response) error {
	srv := daemon.NewServer(daemon.Options{
		SocketPath: socketPath,
		LockPath:   lockPath,
		Handler:    handler,
	})
	if err := srv.Start(ctx); err != nil {
		return err
	}
	<-srv.Done()
	return nil
}

func main() {
	ctx, stop := daemon.ContextWithSignals(context.Background())
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, defaultDependencies()))
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, deps dependencies) int {
	deps = normalizeDeps(deps)
	if len(args) == 0 {
		writeErrorResponse(stdout, nil, protocol.ErrorInvalidConfig, "missing command")
		return 0
	}
	switch args[0] {
	case "version":
		fmt.Fprintln(stdout, "omg dev")
		return 0
	case "reload":
		writeResponse(stdout, protocol.Response{OK: true, Action: protocol.ActionNone, Output: "", Warnings: []string{"reload is a v1 stub"}, Error: nil})
		return 0
	case "on-submit":
		return runOnSubmit(ctx, stdin, stdout, stderr, deps)
	case "on-tool-before":
		return runOnToolBefore(ctx, stdin, stdout, stderr, deps)
	case "run":
		return runRun(ctx, args[1:], stdin, stdout, stderr, deps)
	case "daemon":
		if len(args) < 2 {
			writeErrorResponse(stdout, nil, protocol.ErrorInvalidConfig, "missing daemon subcommand")
			return 0
		}
		switch args[1] {
		case "start":
			return runDaemonStart(ctx, stdout, stderr, deps)
		case "stop":
			return runDaemonStop(ctx, stdout, deps)
		default:
			writeErrorResponse(stdout, nil, protocol.ErrorInvalidConfig, "unknown daemon subcommand")
			return 0
		}
	default:
		writeErrorResponse(stdout, nil, protocol.ErrorInvalidConfig, "unknown command")
		return 0
	}
}

func runOnSubmit(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, deps dependencies) int {
	req, ok := readRequestToStdout(stdin, stdout)
	if !ok {
		return 0
	}
	resp, err := deps.client.RoundTrip(ctx, *req)
	if err == nil {
		writeResponse(stdout, resp)
		return 0
	}
	if deps.stderrDiag {
		fmt.Fprintf(stderr, "socket unavailable: %v\n", err)
	}
	resp = inlineSubmitFallback(ctx, req, deps, err)
	writeResponse(stdout, resp)
	return 0
}

func runOnToolBefore(ctx context.Context, stdin io.Reader, stdout, stderr io.Writer, deps dependencies) int {
	req, ok := readRequestToStdout(stdin, stdout)
	if !ok {
		return 0
	}
	resp, err := deps.client.RoundTrip(ctx, *req)
	if err == nil {
		writeResponse(stdout, resp)
		return 0
	}
	if deps.stderrDiag {
		fmt.Fprintf(stderr, "socket unavailable: %v\n", err)
	}
	writeResponse(stdout, protocol.OKResponse(req, protocol.ActionWarn, "", []string{"socket unavailable: " + err.Error(), "on-tool-before stub fallback"}))
	return 0
}

func runRun(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, deps dependencies) int {
	_ = stdin
	var checks []string
	var goalParts []string
	for i := 0; i < len(args); i++ {
		if args[i] == "--check" && i+1 < len(args) {
			checks = append(checks, args[i+1])
			i++
			continue
		}
		goalParts = append(goalParts, args[i])
	}
	if len(goalParts) == 0 {
		writeErrorResponse(stdout, nil, protocol.ErrorInvalidConfig, "missing goal: usage: omg run \"<goal>\" [--check <kind>:<arg>[:<expected>]]")
		return 1
	}
	goal := strings.Join(goalParts, " ")
	payload := map[string]any{"goal": goal}
	if len(checks) > 0 {
		payload["checks"] = checks
	}
	req := protocol.Request{
		Version:   protocol.Version,
		RequestID: newRequestID(),
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Project:   deps.cwd,
		CWD:       deps.cwd,
		Payload:   payload,
	}
	resp, err := deps.client.RoundTrip(ctx, req)
	if err == nil {
		writeResponse(stdout, resp)
		if !resp.OK {
			return 1
		}
		return 0
	}
	if deps.stderrDiag {
		fmt.Fprintf(stderr, "socket unavailable: %v\n", err)
	}
	return runInlineFallback(ctx, &req, stdout, stderr, deps)
}

func runInlineFallback(ctx context.Context, req *protocol.Request, stdout, stderr io.Writer, deps dependencies) int {
	cfg, cfgErr := config.Load(configOptions(deps))
	if cfgErr != nil {
		writeResponse(stdout, protocol.ErrorResponse(req, protocol.ActionWarn,
			protocol.NewProtocolError(protocol.ErrorInvalidConfig, cfgErr.Error(), false), nil))
		return 1
	}
	storageSink, storageErr := buildStorageSink(ctx, cfg, deps)
	if storageErr != nil && deps.stderrDiag {
		fmt.Fprintf(stderr, "storage unavailable: %v\n", storageErr)
	}
	if storageSink != nil {
		defer func() { _ = storageSink.Shutdown(context.Background()) }()
	}
	resp := orchestrator.Handle(ctx, req, orchestrator.Dependencies{
		ConfigOptions: configOptions(deps),
		StorageSink:   storageSink,
		WorkerFactory: deps.workerFactory,
	})
	writeResponse(stdout, resp)
	if !resp.OK {
		return 1
	}
	return 0
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "req-fallback"
	}
	return "req-" + hex.EncodeToString(b)
}

func runDaemonStop(ctx context.Context, stdout io.Writer, deps dependencies) int {
	req := protocol.Request{Version: protocol.Version, RequestID: "daemon-stop", Provider: protocol.ProviderDirect, Event: protocol.EventOnStop, Project: deps.cwd, CWD: deps.cwd, TimeoutMS: 300}
	resp, err := deps.client.RoundTrip(ctx, req)
	if err != nil {
		writeResponse(stdout, protocol.ErrorResponse(&req, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorSocketUnavailable, err.Error(), false), nil))
		return 0
	}
	writeResponse(stdout, resp)
	return 0
}

func runDaemonStart(ctx context.Context, stdout, stderr io.Writer, deps dependencies) int {
	cfg, err := config.Load(configOptions(deps))
	if err != nil {
		fmt.Fprintf(stderr, "config validation failed: %v\n", err)
		return 1
	}
	if _, err := registry.Build(cfg, registry.Options{}); err != nil {
		fmt.Fprintf(stderr, "registry validation failed: %v\n", err)
		return 1
	}
	storageSink, err := buildStorageSink(context.Background(), cfg, deps)
	if err != nil {
		fmt.Fprintf(stderr, "storage validation failed: %v\n", err)
		return 1
	}
	if storageSink != nil {
		defer func() { _ = storageSink.Shutdown(context.Background()) }()
	}
	stopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	handler := func(ctx context.Context, req *protocol.Request) protocol.Response {
		resp := orchestrator.Handle(ctx, req, orchestrator.Dependencies{ConfigOptions: configOptions(deps), StorageSink: storageSink})
		if req != nil && req.Event == protocol.EventOnStop {
			cancel()
		}
		return resp
	}
	if err := deps.starter.Start(stopCtx, defaultSocketPath(), defaultLockPath(), handler); err != nil {
		fmt.Fprintf(stderr, "daemon start failed: %v\n", err)
		return 1
	}
	_ = stdout
	return 0
}

func buildStorageSink(ctx context.Context, cfg config.Config, deps dependencies) (managedStorageSink, error) {
	factory := deps.newSink
	if factory == nil {
		factory = func(ctx context.Context, projectRoot string, options storage.Options) (managedStorageSink, error) {
			return storage.OpenStore(ctx, projectRoot, options)
		}
	}
	return factory(ctx, deps.cwd, storage.Options{DBPath: cfg.Storage.Database.Path})
}

func inlineSubmitFallback(ctx context.Context, req *protocol.Request, deps dependencies, socketErr error) protocol.Response {
	cfg, err := config.Load(configOptions(deps))
	if err != nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorInvalidConfig, err.Error(), false), []string{"socket unavailable: " + socketErr.Error()})
	}
	reg, err := registry.Build(cfg, registry.Options{})
	if err != nil {
		return protocol.ErrorResponse(req, protocol.ActionWarn, protocol.NewProtocolError(protocol.ErrorInvalidConfig, err.Error(), false), []string{"socket unavailable: " + socketErr.Error()})
	}
	input := payloadText(req)
	result := route.Route(input, reg)
	warning := fmt.Sprintf("inline fallback: procedure=%s domain=%s agent=%s confidence=%s", result.Procedure, result.Domain, result.Agent, result.Confidence)
	_ = ctx
	return protocol.OKResponse(req, protocol.ActionWarn, "", []string{"socket unavailable: " + socketErr.Error(), warning})
}

func payloadText(req *protocol.Request) string {
	if req == nil || req.Payload == nil {
		return ""
	}
	for _, key := range []string{"input", "prompt", "text"} {
		if value, ok := req.Payload[key].(string); ok {
			return value
		}
	}
	return ""
}

func readRequestToStdout(stdin io.Reader, stdout io.Writer) (*protocol.Request, bool) {
	req, perr := protocol.ReadRequest(stdin)
	if perr != nil {
		writeResponse(stdout, protocol.ErrorResponse(req, protocol.ActionWarn, perr, nil))
		return nil, false
	}
	return req, true
}

func writeErrorResponse(stdout io.Writer, req *protocol.Request, class protocol.ErrorClass, message string) {
	writeResponse(stdout, protocol.ErrorResponse(req, protocol.ActionWarn, protocol.NewProtocolError(class, message, false), nil))
}

func writeResponse(stdout io.Writer, resp protocol.Response) {
	_ = protocol.WriteResponse(stdout, resp)
}

func configOptions(deps dependencies) config.Options {
	return config.Options{BuiltInDir: "", UserDir: filepath.Join(deps.home, ".omg"), ProjectDir: filepath.Join(deps.cwd, ".omg")}
}

func normalizeDeps(deps dependencies) dependencies {
	if deps.cwd == "" {
		cwd, err := os.Getwd()
		if err == nil {
			deps.cwd = cwd
		}
	}
	if deps.home == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			deps.home = home
		}
	}
	if deps.client == nil {
		deps.client = unixClient{socketPath: defaultSocketPath(), timeout: 100 * time.Millisecond}
	}
	if deps.starter == nil {
		deps.starter = daemonStarter{}
	}
	if deps.workerFactory == nil {
		deps.workerFactory = func(ctx context.Context, cfg config.WorkerConfig) (adapter.Worker, error) {
			return pi.New(cfg)
		}
	}
	return deps
}

func defaultDependencies() dependencies {
	return normalizeDeps(dependencies{stderrDiag: true})
}

func defaultSocketPath() string { return filepath.Join(os.TempDir(), "omg.sock") }
func defaultLockPath() string   { return filepath.Join(os.TempDir(), "omg.lock") }

func (c unixClient) RoundTrip(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "unix", c.socketPath)
	if err != nil {
		return protocol.Response{}, err
	}
	defer conn.Close()
	if c.timeout > 0 {
		_ = conn.SetDeadline(time.Now().Add(c.timeout))
	}
	if err := writeRequest(conn, req); err != nil {
		return protocol.Response{}, err
	}
	var resp protocol.Response
	if err := protocolDecode(conn, &resp); err != nil {
		return protocol.Response{}, err
	}
	return resp, nil
}

func writeRequest(w io.Writer, req protocol.Request) error {
	data, err := marshalRequest(req)
	if err != nil {
		return err
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

func marshalRequest(req protocol.Request) ([]byte, error) {
	return json.Marshal(req)
}

func protocolDecode(r io.Reader, resp *protocol.Response) error {
	return json.NewDecoder(r).Decode(resp)
}
