package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"oh-my-gunnsama/internal/protocol"
)

func TestServerListenAcceptRespondCycle(t *testing.T) {
	dir := daemonTestDir(t)
	server := NewServer(Options{
		SocketPath: filepath.Join(dir, "omg.sock"),
		LockPath:   filepath.Join(dir, "omg.lock"),
		Handler: func(ctx context.Context, req *protocol.Request) protocol.Response {
			return protocol.OKResponse(req, protocol.ActionInjectPrompt, "inline prompt", nil)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer server.Close()

	resp := roundTrip(t, server.SocketPath(), protocol.Request{
		Version:   protocol.Version,
		RequestID: "req-cycle",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnSubmit,
		Project:   dir,
		CWD:       dir,
	})

	if !resp.OK || resp.RequestID != "req-cycle" || resp.Action != protocol.ActionInjectPrompt || resp.Output != "inline prompt" {
		t.Fatalf("unexpected response: %#v", resp)
	}
}

func TestServerTimesOutSlowRequest(t *testing.T) {
	dir := daemonTestDir(t)
	server := NewServer(Options{
		SocketPath: filepath.Join(dir, "omg.sock"),
		LockPath:   filepath.Join(dir, "omg.lock"),
		Timeouts: TimeoutTiers{
			OnToolBefore: 25 * time.Millisecond,
		},
		Handler: func(ctx context.Context, req *protocol.Request) protocol.Response {
			<-ctx.Done()
			return protocol.OKResponse(req, protocol.ActionNone, "too late", nil)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	defer server.Close()

	resp := roundTrip(t, server.SocketPath(), protocol.Request{
		Version:   protocol.Version,
		RequestID: "req-timeout",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnToolBefore,
		Project:   dir,
		CWD:       dir,
	})

	if resp.OK {
		t.Fatalf("timeout response should fail: %#v", resp)
	}
	if resp.RequestID != "req-timeout" {
		t.Fatalf("timeout response did not echo request_id: %#v", resp)
	}
	if resp.Error == nil || resp.Error.Class != protocol.ErrorTimeout {
		t.Fatalf("expected structured timeout error: %#v", resp.Error)
	}
}

func TestServerRejectsDuplicateLock(t *testing.T) {
	dir := daemonTestDir(t)
	first := NewServer(Options{
		SocketPath: filepath.Join(dir, "omg.sock"),
		LockPath:   filepath.Join(dir, "omg.lock"),
		Handler: func(ctx context.Context, req *protocol.Request) protocol.Response {
			return protocol.OKResponse(req, protocol.ActionNone, "", nil)
		},
	})
	second := NewServer(Options{
		SocketPath: filepath.Join(dir, "omg-2.sock"),
		LockPath:   filepath.Join(dir, "omg.lock"),
		Handler: func(ctx context.Context, req *protocol.Request) protocol.Response {
			return protocol.OKResponse(req, protocol.ActionNone, "", nil)
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := first.Start(ctx); err != nil {
		t.Fatalf("first Start returned error: %v", err)
	}
	defer first.Close()

	if err := second.Start(ctx); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Start error = %v, want ErrAlreadyRunning", err)
	}
}

func roundTrip(t *testing.T, socketPath string, req protocol.Request) protocol.Response {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("dial unix socket: %v", err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		t.Fatalf("encode request: %v", err)
	}

	var resp protocol.Response
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return resp
}

func daemonTestDir(t *testing.T) string {
	t.Helper()
	base := filepath.Join("..", "..", ".omg", "tmp")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("create daemon test base: %v", err)
	}
	dir, err := os.MkdirTemp(base, "daemon-")
	if err != nil {
		t.Fatalf("create daemon test dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("abs daemon test dir: %v", err)
	}
	return abs
}
