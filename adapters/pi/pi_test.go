package pi

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
)

func absFixture(t *testing.T, rel string) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("testdata", rel))
	if err != nil {
		t.Fatalf("filepath.Abs: %v", err)
	}
	return p
}

func newDriver(t *testing.T, fixture string) *Driver {
	t.Helper()
	cfg := config.WorkerConfig{Kind: "pi", BinaryPath: absFixture(t, fixture)}
	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return d
}

func drainEvents(t *testing.T, ch <-chan adapter.WorkerEvent, timeout time.Duration) ([]adapter.WorkerEvent, bool) {
	t.Helper()
	var events []adapter.WorkerEvent
	deadline := time.After(timeout)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return events, true
			}
			events = append(events, evt)
		case <-deadline:
			return events, false
		}
	}
}

func TestDriver_HappyPath(t *testing.T) {
	d := newDriver(t, "fake_pi.sh")
	spec := adapter.WorkerSpec{
		Goal:       "test",
		AllowTools: []string{"write"},
	}
	if err := d.Spawn(context.Background(), spec); err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	events, closed := drainEvents(t, d.Events(), 5*time.Second)
	if !closed {
		t.Fatalf("events channel did not close within 5s; received %d events", len(events))
	}
	if got := len(events); got < 4 {
		t.Fatalf("got %d events, want >= 4: %+v", got, events)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	result, err := d.Wait(waitCtx)
	if err != nil {
		t.Fatalf("Wait returned error: %v", err)
	}
	if !result.Success {
		t.Fatalf("result.Success = false, want true (reason=%q)", result.Reason)
	}
	if result.ExitCode != 0 {
		t.Fatalf("result.ExitCode = %d, want 0", result.ExitCode)
	}

	var sawToolStart bool
	for _, e := range events {
		if e.Kind == "tool_execution_start" && e.ToolName == "write" {
			sawToolStart = true
			break
		}
	}
	if !sawToolStart {
		t.Fatalf("no tool_execution_start event with ToolName=write among %d events", len(events))
	}
}

func TestDriver_PassesAPIKeyFromSpecEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	argsPath := filepath.Join(t.TempDir(), "args.txt")
	d := newDriver(t, "fake_pi_args.sh")
	spec := adapter.WorkerSpec{
		Goal: "test",
		Env: map[string]string{
			"ANTHROPIC_API_KEY": "spec-key",
			"FAKE_PI_ARGS_PATH": argsPath,
		},
	}
	if err := d.Spawn(context.Background(), spec); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if _, closed := drainEvents(t, d.Events(), 2*time.Second); !closed {
		t.Fatal("events channel did not close within 2s")
	}
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	if _, err := d.Wait(waitCtx); err != nil {
		t.Fatalf("Wait: %v", err)
	}

	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatalf("ReadFile(args): %v", err)
	}
	args := string(raw)
	if !strings.Contains(args, "--api-key spec-key") {
		t.Fatalf("args = %q, want --api-key from WorkerSpec.Env", args)
	}
}

func TestDriver_NonZeroExit(t *testing.T) {
	d := newDriver(t, "fake_pi_fail.sh")
	if err := d.Spawn(context.Background(), adapter.WorkerSpec{Goal: "test"}); err != nil {
		t.Fatalf("Spawn: %v", err)
	}
	if _, closed := drainEvents(t, d.Events(), 2*time.Second); !closed {
		t.Fatal("events channel did not close within 2s for failing fixture")
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	result, _ := d.Wait(waitCtx)
	if result.Success {
		t.Fatal("result.Success = true, want false for failing fixture")
	}
	if result.ExitCode != 1 {
		t.Fatalf("result.ExitCode = %d, want 1", result.ExitCode)
	}
	if !strings.Contains(result.Reason, "simulated failure") {
		t.Fatalf("result.Reason = %q, want substring 'simulated failure'", result.Reason)
	}
}

func TestDriver_ContextCancel(t *testing.T) {
	d := newDriver(t, "fake_pi_sleep.sh")
	ctx, cancel := context.WithCancel(context.Background())
	if err := d.Spawn(ctx, adapter.WorkerSpec{Goal: "test"}); err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	eventsClosed := make(chan struct{})
	go func() {
		for range d.Events() {
		}
		close(eventsClosed)
	}()

	time.AfterFunc(50*time.Millisecond, cancel)

	select {
	case <-eventsClosed:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("events channel did not close within 500ms after ctx cancel")
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer waitCancel()
	result, _ := d.Wait(waitCtx)
	if result.Success {
		t.Fatal("result.Success = true, want false after context cancel")
	}
}

func TestDriver_DoubleSpawnRejected(t *testing.T) {
	d := newDriver(t, "fake_pi.sh")
	if err := d.Spawn(context.Background(), adapter.WorkerSpec{Goal: "test"}); err != nil {
		t.Fatalf("first Spawn: %v", err)
	}

	err := d.Spawn(context.Background(), adapter.WorkerSpec{Goal: "again"})
	if err == nil {
		t.Fatal("second Spawn returned nil error, want 'already spawned'")
	}
	if !strings.Contains(err.Error(), "already spawned") {
		t.Fatalf("err = %q, want substring 'already spawned'", err)
	}

	for range d.Events() {
	}
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	_, _ = d.Wait(waitCtx)
}

func TestDriver_AbortKillsProcess(t *testing.T) {
	d := newDriver(t, "fake_pi_sleep.sh")
	if err := d.Spawn(context.Background(), adapter.WorkerSpec{Goal: "test"}); err != nil {
		t.Fatalf("Spawn: %v", err)
	}

	eventsClosed := make(chan struct{})
	go func() {
		for range d.Events() {
		}
		close(eventsClosed)
	}()

	time.AfterFunc(50*time.Millisecond, func() {
		if err := d.Abort(context.Background()); err != nil {
			t.Errorf("Abort: %v", err)
		}
	})

	select {
	case <-eventsClosed:
	case <-time.After(2 * time.Second):
		t.Fatal("events channel did not close within 2s after Abort")
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()
	result, _ := d.Wait(waitCtx)
	if result.ExitCode == 0 {
		t.Fatalf("result.ExitCode = 0, want non-zero after Abort")
	}
}
