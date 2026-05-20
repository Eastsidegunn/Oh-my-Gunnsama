package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/storage"
)

type stubWorker struct {
	events  []adapter.WorkerEvent
	result  adapter.WorkerResult
	waitErr error
	ch      chan adapter.WorkerEvent
	done    chan struct{}
}

func (s *stubWorker) Spawn(ctx context.Context, spec adapter.WorkerSpec) error {
	s.ch = make(chan adapter.WorkerEvent, len(s.events))
	s.done = make(chan struct{})
	go func() {
		for _, evt := range s.events {
			select {
			case s.ch <- evt:
			case <-ctx.Done():
				close(s.ch)
				close(s.done)
				return
			}
		}
		close(s.ch)
		close(s.done)
	}()
	return nil
}

func (s *stubWorker) Events() <-chan adapter.WorkerEvent { return s.ch }

func (s *stubWorker) Wait(ctx context.Context) (adapter.WorkerResult, error) {
	if s.done != nil {
		select {
		case <-s.done:
		case <-ctx.Done():
			return adapter.WorkerResult{}, ctx.Err()
		}
	}
	return s.result, s.waitErr
}

func (s *stubWorker) Abort(ctx context.Context) error { return nil }

func newStubFactory(w *stubWorker) WorkerFactory {
	return func(ctx context.Context, cfg config.WorkerConfig) (adapter.Worker, error) {
		return w, nil
	}
}

func canonicalRunEvents() []adapter.WorkerEvent {
	base := time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC)
	return []adapter.WorkerEvent{
		{Kind: "agent_start", Timestamp: base, Sequence: 1},
		{Kind: "tool_execution_start", Timestamp: base.Add(1 * time.Second), ToolName: "read", Sequence: 2},
		{Kind: "tool_execution_end", Timestamp: base.Add(2 * time.Second), ToolName: "read", Message: "ok", Sequence: 3},
		{Kind: "agent_end", Timestamp: base.Add(3 * time.Second), Sequence: 4},
	}
}

func eventTypeCounts(events []storage.Event) map[storage.EventType]int {
	counts := map[storage.EventType]int{}
	for _, e := range events {
		counts[e.Type]++
	}
	return counts
}

func TestHandleRun_HappyPath(t *testing.T) {
	sink := &recordingSink{}
	worker := &stubWorker{
		events: canonicalRunEvents(),
		result: adapter.WorkerResult{ExitCode: 0, Success: true},
	}
	deps := Dependencies{
		StorageSink:   sink,
		WorkerFactory: newStubFactory(worker),
		Now:           func() time.Time { return time.Unix(0, 0).UTC() },
		OwnerID:       "test",
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-1",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload:   map[string]any{"goal": "say hello"},
	}

	resp := Handle(context.Background(), req, deps)

	if !resp.OK {
		t.Fatalf("expected OK=true, got %#v", resp)
	}
	if resp.Action != protocol.ActionNone {
		t.Fatalf("expected ActionNone, got %q", resp.Action)
	}
	for _, want := range []string{"run_id=", "work_id=", "session_id=", "status=finished", "events=4"} {
		if !strings.Contains(resp.Output, want) {
			t.Fatalf("output missing %q: %s", want, resp.Output)
		}
	}

	counts := eventTypeCounts(sink.events)
	required := map[storage.EventType]int{
		storage.EventRunCreated:        1,
		storage.EventWorkCreated:       1,
		storage.EventWorkStarted:       1,
		storage.EventWorkCompleted:     1,
		storage.EventAgentSpawned:      1,
		storage.EventToolCallStarted:   1,
		storage.EventToolCallCompleted: 1,
		storage.EventAgentStopped:      1,
	}
	for et, want := range required {
		if counts[et] < want {
			t.Fatalf("event %s count = %d, want >= %d (all=%v)", et, counts[et], want, counts)
		}
	}
	if counts[storage.EventAgentHeartbeat] < 1 {
		t.Fatalf("expected >=1 EventAgentHeartbeat, got %d", counts[storage.EventAgentHeartbeat])
	}
	if counts[storage.EventRunStatusChanged] < 2 {
		t.Fatalf("expected >=2 EventRunStatusChanged (executing+completed), got %d", counts[storage.EventRunStatusChanged])
	}

	var sawExecuting, sawCompleted bool
	for _, e := range sink.events {
		if e.Type != storage.EventRunStatusChanged {
			continue
		}
		if e.Status == "executing" {
			sawExecuting = true
		}
		if e.Status == "completed" {
			sawCompleted = true
		}
	}
	if !sawExecuting || !sawCompleted {
		t.Fatalf("expected both executing and completed status changes; executing=%v completed=%v", sawExecuting, sawCompleted)
	}

	workIDs := map[string]bool{}
	for _, e := range sink.events {
		if e.WorkID != "" {
			workIDs[e.WorkID] = true
		}
	}
	if len(workIDs) != 1 {
		t.Fatalf("expected exactly 1 unique WorkID across events, got %d: %v", len(workIDs), workIDs)
	}

	for _, e := range sink.events {
		if e.Type == storage.EventToolCallStarted || e.Type == storage.EventToolCallCompleted {
			if e.WorkID == "" {
				t.Fatalf("expected WorkID on tool_call event %s, got empty; event=%#v", e.Type, e)
			}
		}
	}
}

func TestHandleRun_WorkerFailure(t *testing.T) {
	sink := &recordingSink{}
	worker := &stubWorker{
		events: []adapter.WorkerEvent{{Kind: "agent_start", Timestamp: time.Unix(0, 0).UTC(), Sequence: 1}},
		result: adapter.WorkerResult{ExitCode: 1, Success: false, Reason: "model error"},
	}
	deps := Dependencies{
		StorageSink:   sink,
		WorkerFactory: newStubFactory(worker),
		Now:           func() time.Time { return time.Unix(0, 0).UTC() },
		OwnerID:       "test",
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-2",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload:   map[string]any{"goal": "say hello"},
	}

	resp := Handle(context.Background(), req, deps)

	if !resp.OK {
		t.Fatalf("expected OK=true (failed run still returns OK), got %#v", resp)
	}
	if resp.Action != protocol.ActionWarn {
		t.Fatalf("expected ActionWarn, got %q", resp.Action)
	}
	if !strings.Contains(resp.Output, "status=failed") {
		t.Fatalf("output missing status=failed: %s", resp.Output)
	}
	if !strings.Contains(resp.Output, "events=1") {
		t.Fatalf("output missing events=1: %s", resp.Output)
	}

	var sawFailedStatus bool
	for _, e := range sink.events {
		if e.Type == storage.EventRunStatusChanged && e.Status == "failed" {
			sawFailedStatus = true
			break
		}
	}
	if !sawFailedStatus {
		t.Fatalf("expected EventRunStatusChanged with status=failed; events=%v", sink.events)
	}

	var sawWorkFailed, sawWorkCompleted bool
	for _, e := range sink.events {
		if e.Type == storage.EventWorkFailed {
			sawWorkFailed = true
		}
		if e.Type == storage.EventWorkCompleted {
			sawWorkCompleted = true
		}
	}
	if !sawWorkFailed {
		t.Fatalf("expected EventWorkFailed event on worker failure; events=%v", sink.events)
	}
	if sawWorkCompleted {
		t.Fatalf("did NOT expect EventWorkCompleted on worker failure (got both); events=%v", sink.events)
	}

	var sawFailWarning bool
	for _, w := range resp.Warnings {
		if strings.Contains(w, "run failed") && strings.Contains(w, "model error") {
			sawFailWarning = true
			break
		}
	}
	if !sawFailWarning {
		t.Fatalf("expected warning containing 'run failed: model error'; warnings=%v", resp.Warnings)
	}
}

func TestHandleRun_VerificationPasses(t *testing.T) {
	sink := &recordingSink{}
	worker := &stubWorker{
		events: canonicalRunEvents(),
		result: adapter.WorkerResult{ExitCode: 0, Success: true},
	}
	deps := Dependencies{
		StorageSink:   sink,
		WorkerFactory: newStubFactory(worker),
		Now:           func() time.Time { return time.Unix(0, 0).UTC() },
		OwnerID:       "test",
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-verify-pass",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload: map[string]any{
			"goal":   "say hello",
			"checks": []string{"bash:true"},
		},
	}

	resp := Handle(context.Background(), req, deps)

	if !resp.OK {
		t.Fatalf("expected OK=true, got %#v", resp)
	}
	if resp.Action != protocol.ActionNone {
		t.Fatalf("expected ActionNone (verification passed), got %q", resp.Action)
	}
	if !strings.Contains(resp.Output, "verification=verified") {
		t.Fatalf("output missing 'verification=verified': %s", resp.Output)
	}
	if !strings.Contains(resp.Output, "status=finished") {
		t.Fatalf("output missing 'status=finished': %s", resp.Output)
	}

	counts := eventTypeCounts(sink.events)
	if counts[storage.EventVerificationPassed] < 1 {
		t.Fatalf("expected >=1 EventVerificationPassed, got %d (all=%v)", counts[storage.EventVerificationPassed], counts)
	}
	if counts[storage.EventVerificationFailed] != 0 {
		t.Fatalf("expected 0 EventVerificationFailed, got %d", counts[storage.EventVerificationFailed])
	}
}

func TestHandleRun_VerificationFails(t *testing.T) {
	sink := &recordingSink{}
	worker := &stubWorker{
		events: canonicalRunEvents(),
		result: adapter.WorkerResult{ExitCode: 0, Success: true},
	}
	deps := Dependencies{
		StorageSink:   sink,
		WorkerFactory: newStubFactory(worker),
		Now:           func() time.Time { return time.Unix(0, 0).UTC() },
		OwnerID:       "test",
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-verify-fail",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload: map[string]any{
			"goal":   "say hello",
			"checks": []string{"bash:false"},
		},
	}

	resp := Handle(context.Background(), req, deps)

	if !resp.OK {
		t.Fatalf("expected OK=true (verification failure is soft), got %#v", resp)
	}
	if resp.Action != protocol.ActionWarn {
		t.Fatalf("expected ActionWarn (verification failed), got %q", resp.Action)
	}
	if !strings.Contains(resp.Output, "verification=failed") {
		t.Fatalf("output missing 'verification=failed': %s", resp.Output)
	}
	if !strings.Contains(resp.Output, "status=failed") {
		t.Fatalf("output missing 'status=failed' (run lifecycle): %s", resp.Output)
	}

	counts := eventTypeCounts(sink.events)
	if counts[storage.EventVerificationFailed] < 1 {
		t.Fatalf("expected >=1 EventVerificationFailed, got %d (all=%v)", counts[storage.EventVerificationFailed], counts)
	}
	if counts[storage.EventVerificationPassed] != 0 {
		t.Fatalf("expected 0 EventVerificationPassed, got %d", counts[storage.EventVerificationPassed])
	}
}

func TestHandleRun_VerificationExecutionErrorFailsRun(t *testing.T) {
	sink := &rejectingSink{
		failOn: storage.EventVerificationPassed,
	}
	worker := &stubWorker{
		events: canonicalRunEvents(),
		result: adapter.WorkerResult{ExitCode: 0, Success: true},
	}
	deps := Dependencies{
		StorageSink:   sink,
		WorkerFactory: newStubFactory(worker),
		Now:           func() time.Time { return time.Unix(0, 0).UTC() },
		OwnerID:       "test",
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-verify-exec-err",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload: map[string]any{
			"goal":   "say hello",
			"checks": []string{"bash:true"},
		},
	}

	resp := Handle(context.Background(), req, deps)

	if !resp.OK {
		t.Fatalf("expected OK=true (soft failure), got %#v", resp)
	}
	if resp.Action != protocol.ActionWarn {
		t.Fatalf("expected ActionWarn (verification could not execute), got %q", resp.Action)
	}
	if !strings.Contains(resp.Output, "status=failed") {
		t.Fatalf("expected run lifecycle to be failed when verification cannot execute, got: %s", resp.Output)
	}
	if !strings.Contains(resp.Output, "verification=failed") {
		t.Fatalf("expected verification=failed when verifier cannot run, got: %s", resp.Output)
	}

	var sawExecutionErrorWarning bool
	for _, w := range resp.Warnings {
		if strings.Contains(w, "verification execution error") {
			sawExecutionErrorWarning = true
			break
		}
	}
	if !sawExecutionErrorWarning {
		t.Fatalf("expected warning containing 'verification execution error'; warnings=%v", resp.Warnings)
	}
}

func TestHandleRun_VerificationSkippedWithoutChecks(t *testing.T) {
	sink := &recordingSink{}
	worker := &stubWorker{
		events: canonicalRunEvents(),
		result: adapter.WorkerResult{ExitCode: 0, Success: true},
	}
	deps := Dependencies{
		StorageSink:   sink,
		WorkerFactory: newStubFactory(worker),
		Now:           func() time.Time { return time.Unix(0, 0).UTC() },
		OwnerID:       "test",
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-no-checks",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload:   map[string]any{"goal": "say hello"},
	}

	resp := Handle(context.Background(), req, deps)

	if !resp.OK {
		t.Fatalf("expected OK=true, got %#v", resp)
	}
	if !strings.Contains(resp.Output, "verification=skipped") {
		t.Fatalf("output missing 'verification=skipped': %s", resp.Output)
	}
	counts := eventTypeCounts(sink.events)
	if counts[storage.EventVerificationPassed]+counts[storage.EventVerificationFailed] != 0 {
		t.Fatalf("expected 0 verification events (no checks), got passed=%d failed=%d", counts[storage.EventVerificationPassed], counts[storage.EventVerificationFailed])
	}
}

func TestHandleRun_MissingGoal(t *testing.T) {
	deps := Dependencies{
		WorkerFactory: newStubFactory(&stubWorker{}),
		Now:           func() time.Time { return time.Unix(0, 0).UTC() },
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-3",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload:   nil,
	}

	resp := Handle(context.Background(), req, deps)

	if resp.OK {
		t.Fatalf("expected OK=false, got %#v", resp)
	}
	if resp.Error == nil || resp.Error.Class != protocol.ErrorInvalidConfig {
		t.Fatalf("expected error class=invalid_config, got %#v", resp.Error)
	}

	reqEmptyGoal := *req
	reqEmptyGoal.Payload = map[string]any{"goal": "   "}
	resp2 := Handle(context.Background(), &reqEmptyGoal, deps)
	if resp2.OK || resp2.Error == nil || resp2.Error.Class != protocol.ErrorInvalidConfig {
		t.Fatalf("expected invalid_config for whitespace goal, got %#v", resp2)
	}
}

func TestHandleRun_NoWorkerFactory(t *testing.T) {
	deps := Dependencies{
		Now: func() time.Time { return time.Unix(0, 0).UTC() },
	}
	req := &Request{
		Version:   protocol.Version,
		RequestID: "run-4",
		Provider:  protocol.ProviderDirect,
		Event:     protocol.EventOnRun,
		Payload:   map[string]any{"goal": "say hello"},
	}

	resp := Handle(context.Background(), req, deps)

	if resp.OK {
		t.Fatalf("expected OK=false, got %#v", resp)
	}
	if resp.Error == nil || resp.Error.Class != protocol.ErrorInvalidConfig {
		t.Fatalf("expected error class=invalid_config, got %#v", resp.Error)
	}
	if !strings.Contains(resp.Error.Message, "WorkerFactory") {
		t.Fatalf("expected message to mention WorkerFactory, got %q", resp.Error.Message)
	}
}
