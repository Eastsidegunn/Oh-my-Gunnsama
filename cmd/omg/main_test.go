package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/storage"
)

func TestOnSubmitSocketFailureUsesInlineRouteFallback(t *testing.T) {
	root := t.TempDir()
	writeOMGConfig(t, root, `
agents:
  architect:
    description: Architect
    autonomy: cautious
    triggers:
      - domain: architecture
        pattern: architecture|interface
        priority: 10
`)
	deps := testDeps(root)
	deps.client = fakeClient{err: errors.New("socket missing")}
	stdin := requestJSON(t, protocol.Request{Version: protocol.Version, RequestID: "req-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnSubmit, Project: root, CWD: root, Payload: map[string]any{"input": "review architecture interface"}})
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"on-submit"}, strings.NewReader(stdin), &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	resp := decodeResponse(t, stdout.Bytes())
	if !resp.OK || resp.Action != protocol.ActionWarn || resp.RequestID != "req-1" {
		t.Fatalf("response = %#v", resp)
	}
	joined := strings.Join(resp.Warnings, "\n")
	if !strings.Contains(joined, "socket unavailable") || !strings.Contains(joined, "inline fallback: procedure=review domain=architecture agent=architect confidence=high") {
		t.Fatalf("warnings = %#v", resp.Warnings)
	}
	if stdout.String() == "" || strings.Contains(stdout.String(), "socket missing") && resp.Error != nil {
		t.Fatalf("stdout should contain only response JSON, got %q", stdout.String())
	}
}

func TestOnSubmitDecodeFailureWritesJSONError(t *testing.T) {
	deps := testDeps(t.TempDir())
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"on-submit"}, strings.NewReader(`{"version":`), &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	resp := decodeResponse(t, stdout.Bytes())
	if resp.OK || resp.Error == nil || resp.Error.Class != protocol.ErrorInvalidConfig || resp.RequestID != "" {
		t.Fatalf("response = %#v", resp)
	}
}

func TestOnToolBeforeSocketFailureReturnsStubWarn(t *testing.T) {
	root := t.TempDir()
	deps := testDeps(root)
	deps.client = fakeClient{err: errors.New("socket missing")}
	stdin := requestJSON(t, protocol.Request{Version: protocol.Version, RequestID: "tool-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnToolBefore, Project: root, CWD: root, Payload: map[string]any{"tool": "read"}})
	var stdout, stderr bytes.Buffer

	code := run(context.Background(), []string{"on-tool-before"}, strings.NewReader(stdin), &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	resp := decodeResponse(t, stdout.Bytes())
	if !resp.OK || resp.Action != protocol.ActionWarn || !strings.Contains(strings.Join(resp.Warnings, "\n"), "socket unavailable") {
		t.Fatalf("response = %#v", resp)
	}
}

func TestDaemonStopSocketFailureReturnsSocketUnavailable(t *testing.T) {
	deps := testDeps(t.TempDir())
	deps.client = fakeClient{err: errors.New("no daemon")}
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"daemon", "stop"}, strings.NewReader(""), &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("exit code = %d", code)
	}
	resp := decodeResponse(t, stdout.Bytes())
	if resp.OK || resp.Error == nil || resp.Error.Class != protocol.ErrorSocketUnavailable {
		t.Fatalf("response = %#v", resp)
	}
}

func TestDaemonStartWiresConfigRegistryAndHandler(t *testing.T) {
	root := t.TempDir()
	writeOMGConfig(t, root, `
agents:
  executor:
    description: Executor
`)
	deps := testDeps(root)
	starter := &fakeStarter{}
	deps.starter = starter
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer

	code := run(ctx, []string{"daemon", "start"}, strings.NewReader(""), &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("daemon start wrote stdout: %q", stdout.String())
	}
	if !starter.started || starter.socketPath != filepath.Join(os.TempDir(), "omg.sock") || starter.lockPath != filepath.Join(os.TempDir(), "omg.lock") {
		t.Fatalf("starter = %#v", starter)
	}
	resp := starter.handler(context.Background(), &protocol.Request{Version: protocol.Version, RequestID: "stop", Event: protocol.EventOnStop})
	if !resp.OK || resp.Action != protocol.ActionNone || !starter.stopTriggered {
		t.Fatalf("stop handler response=%#v stop=%v", resp, starter.stopTriggered)
	}
}

func TestReloadAndVersion(t *testing.T) {
	deps := testDeps(t.TempDir())
	var stdout bytes.Buffer
	if code := run(context.Background(), []string{"version"}, strings.NewReader(""), &stdout, &bytes.Buffer{}, deps); code != 0 || strings.TrimSpace(stdout.String()) != "omg dev" {
		t.Fatalf("version code=%d stdout=%q", code, stdout.String())
	}
	stdout.Reset()
	if code := run(context.Background(), []string{"reload"}, strings.NewReader(""), &stdout, &bytes.Buffer{}, deps); code != 0 {
		t.Fatalf("reload code=%d", code)
	}
	resp := decodeResponse(t, stdout.Bytes())
	if !resp.OK || resp.Action != protocol.ActionNone {
		t.Fatalf("reload response=%#v", resp)
	}
}

type fakeClient struct {
	resp protocol.Response
	err  error
}

func (f fakeClient) RoundTrip(ctx context.Context, req protocol.Request) (protocol.Response, error) {
	if f.err != nil {
		return protocol.Response{}, f.err
	}
	if f.resp.Action == "" {
		return protocol.OKResponse(&req, protocol.ActionNone, "", nil), nil
	}
	return f.resp, nil
}

type fakeStarter struct {
	started       bool
	socketPath    string
	lockPath      string
	handler       func(context.Context, *protocol.Request) protocol.Response
	stopTriggered bool
}

func (f *fakeStarter) Start(ctx context.Context, socketPath, lockPath string, handler func(context.Context, *protocol.Request) protocol.Response) error {
	f.started = true
	f.socketPath = socketPath
	f.lockPath = lockPath
	f.handler = func(ctx context.Context, req *protocol.Request) protocol.Response {
		resp := handler(ctx, req)
		if req.Event == protocol.EventOnStop {
			f.stopTriggered = true
		}
		return resp
	}
	<-ctx.Done()
	return nil
}

func testDeps(root string) dependencies {
	home := filepath.Join(root, "home")
	_ = os.MkdirAll(filepath.Join(home, ".omg"), 0o755)
	return dependencies{
		cwd:        root,
		home:       home,
		client:     fakeClient{err: errors.New("unexpected socket call")},
		starter:    &fakeStarter{},
		stderrDiag: true,
	}
}

func writeOMGConfig(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".omg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .omg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func requestJSON(t *testing.T, req protocol.Request) string {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return string(data)
}

func decodeResponse(t *testing.T, data []byte) protocol.Response {
	t.Helper()
	var resp protocol.Response
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("decode response %q: %v", string(data), err)
	}
	return resp
}

type fakeManagedSink struct {
	events      []storage.Event
	shutdown    bool
	shutdownCtx context.Context
}

func (s *fakeManagedSink) RecordEvent(_ context.Context, event storage.Event) error {
	s.events = append(s.events, event)
	return nil
}

func (s *fakeManagedSink) Shutdown(ctx context.Context) error {
	s.shutdown = true
	s.shutdownCtx = ctx
	return nil
}

func TestDaemonStartWiresCoreDatabaseStore(t *testing.T) {
	root := t.TempDir()
	dbPath := filepath.Join(root, ".omg", "custom.db")
	writeOMGConfig(t, root, `
storage:
  database:
    path: `+dbPath+`
agents:
  executor:
    description: Executor
`)
	deps := testDeps(root)
	starter := &fakeStarter{}
	deps.starter = starter
	fakeSink := &fakeManagedSink{}
	var gotRoot string
	var gotOptions storage.Options
	deps.newSink = func(_ context.Context, projectRoot string, options storage.Options) (managedStorageSink, error) {
		gotRoot = projectRoot
		gotOptions = options
		return fakeSink, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stdout, stderr bytes.Buffer

	code := run(ctx, []string{"daemon", "start"}, strings.NewReader(""), &stdout, &stderr, deps)

	if code != 0 {
		t.Fatalf("exit code = %d stderr=%s", code, stderr.String())
	}
	if gotRoot != root || gotOptions.DBPath != dbPath {
		t.Fatalf("database wiring root=%q options=%#v", gotRoot, gotOptions)
	}
	if !fakeSink.shutdown || fakeSink.shutdownCtx == nil {
		t.Fatalf("storage sink was not shut down")
	}
	resp := starter.handler(context.Background(), &protocol.Request{Version: protocol.Version, RequestID: "tool-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnToolBefore, Project: root, CWD: root, Payload: map[string]any{"tool": "read"}})
	if !resp.OK || len(fakeSink.events) != 1 || fakeSink.events[0].Type != storage.EventToolCallStarted {
		t.Fatalf("resp=%#v events=%#v", resp, fakeSink.events)
	}
}

type captureClient struct {
	response protocol.Response
	captured *protocol.Request
}

func (c *captureClient) RoundTrip(_ context.Context, req protocol.Request) (protocol.Response, error) {
	if c.captured != nil {
		*c.captured = req
	}
	resp := c.response
	if resp.RequestID == "" {
		resp.RequestID = req.RequestID
	}
	return resp, nil
}

type errorClient struct {
	err error
}

func (c *errorClient) RoundTrip(_ context.Context, _ protocol.Request) (protocol.Response, error) {
	return protocol.Response{}, c.err
}

type fakeWorker struct {
	events []adapter.WorkerEvent
	result adapter.WorkerResult
	ch     chan adapter.WorkerEvent
	done   chan struct{}
}

func (w *fakeWorker) Spawn(ctx context.Context, _ adapter.WorkerSpec) error {
	w.ch = make(chan adapter.WorkerEvent, len(w.events))
	w.done = make(chan struct{})
	go func() {
		defer close(w.ch)
		defer close(w.done)
		for _, evt := range w.events {
			select {
			case w.ch <- evt:
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}

func (w *fakeWorker) Events() <-chan adapter.WorkerEvent { return w.ch }

func (w *fakeWorker) Wait(ctx context.Context) (adapter.WorkerResult, error) {
	if w.done == nil {
		return w.result, nil
	}
	select {
	case <-w.done:
		return w.result, nil
	case <-ctx.Done():
		return adapter.WorkerResult{}, ctx.Err()
	}
}

func (w *fakeWorker) Abort(_ context.Context) error { return nil }

func TestRunCmd_BuildsCorrectRequest(t *testing.T) {
	var captured protocol.Request
	fake := &captureClient{
		response: protocol.Response{OK: true, Action: protocol.ActionNone, Output: "run_id=test status=finished events=3", Warnings: []string{}},
		captured: &captured,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	deps := dependencies{
		cwd:    t.TempDir(),
		home:   t.TempDir(),
		client: fake,
	}
	code := run(context.Background(), []string{"run", "make", "hello.txt"}, strings.NewReader(""), &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("exit code = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if captured.Event != protocol.EventOnRun {
		t.Fatalf("Event = %q, want %q", captured.Event, protocol.EventOnRun)
	}
	if captured.Provider != protocol.ProviderDirect {
		t.Fatalf("Provider = %q, want %q", captured.Provider, protocol.ProviderDirect)
	}
	goal, _ := captured.Payload["goal"].(string)
	if goal != "make hello.txt" {
		t.Fatalf("Payload[goal] = %q, want %q", goal, "make hello.txt")
	}
	if !strings.HasPrefix(captured.RequestID, "req-") {
		t.Fatalf("RequestID = %q, want prefix req-", captured.RequestID)
	}
	if !strings.Contains(stdout.String(), "run_id=test") {
		t.Fatalf("stdout missing canned output: %q", stdout.String())
	}
}

func TestRunCmd_MissingGoal(t *testing.T) {
	deps := dependencies{
		cwd:    t.TempDir(),
		home:   t.TempDir(),
		client: &captureClient{},
	}
	var stdout bytes.Buffer
	code := run(context.Background(), []string{"run"}, strings.NewReader(""), &stdout, &bytes.Buffer{}, deps)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout.String(), "goal") {
		t.Fatalf("expected error mentioning goal, got: %s", stdout.String())
	}
}

func TestRunCmd_InlineFallback(t *testing.T) {
	root := t.TempDir()
	writeOMGConfig(t, root, `
agents:
  executor:
    description: Executor
`)
	fake := &errorClient{err: errors.New("socket unavailable")}
	stubWorker := &fakeWorker{
		events: []adapter.WorkerEvent{
			{Kind: "agent_start", Timestamp: time.Now()},
			{Kind: "agent_end", Timestamp: time.Now()},
		},
		result: adapter.WorkerResult{ExitCode: 0, Success: true},
	}
	workerFactory := func(_ context.Context, _ config.WorkerConfig) (adapter.Worker, error) {
		return stubWorker, nil
	}
	deps := dependencies{
		cwd:           root,
		home:          t.TempDir(),
		client:        fake,
		workerFactory: workerFactory,
		newSink: func(_ context.Context, _ string, _ storage.Options) (managedStorageSink, error) {
			return nil, nil
		},
		stderrDiag: true,
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "do", "stuff"}, strings.NewReader(""), &stdout, &stderr, deps)
	if code != 0 {
		t.Fatalf("inline fallback exit = %d, stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "run_id=") {
		t.Fatalf("expected run_id in output, got: %s", stdout.String())
	}
	if !strings.Contains(stdout.String(), "status=finished") {
		t.Fatalf("expected status=finished in output, got: %s", stdout.String())
	}
	if !strings.Contains(stderr.String(), "socket unavailable") {
		t.Fatalf("expected socket-unavailable diagnostic on stderr, got: %s", stderr.String())
	}
}
