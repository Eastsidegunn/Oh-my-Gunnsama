package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/guard"
	"oh-my-gunnsama/internal/protocol"
	"oh-my-gunnsama/internal/state"
	"oh-my-gunnsama/internal/storage"
)

func TestHandleOnSubmitBuildsInjectPrompt(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `
agents:
  executor:
    description: Executes implementation work
    model: gpt-test
    triggers:
      - domain: implementation
        pattern: implement|build
        priority: 10
`)
	req := &Request{Version: protocol.Version, RequestID: "submit-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnSubmit, Project: root, CWD: root, Payload: map[string]any{"input": "implement orchestrator"}}

	resp := Handle(context.Background(), req, Dependencies{ConfigOptions: config.Options{ProjectDir: filepath.Join(root, ".omg")}})

	if !resp.OK || resp.Action != protocol.ActionInjectPrompt || resp.RequestID != "submit-1" {
		t.Fatalf("response = %#v", resp)
	}
	if !strings.Contains(resp.Output, "executor") || !strings.Contains(resp.Output, "implementation") || !strings.Contains(resp.Output, "execute") {
		t.Fatalf("output did not include resolved route/agent prompt context: %q", resp.Output)
	}
}

func TestHandleOnSubmitInjectsAgentPolicyContext(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, root, `
agents:
  executor:
    description: Executes implementation work
    autonomy: cautious
    tools: [read, grep, edit]
    useWhen:
      - Multi-file implementation work
      - Tests can prove the change
    avoidWhen:
      - Pure research with no edits
    triggers:
      - domain: implementation
        pattern: implement|build|refactor
        priority: 10
`)
	req := &Request{Version: protocol.Version, RequestID: "submit-policy-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnSubmit, Project: root, CWD: root, Payload: map[string]any{"input": "refactor orchestrator"}}

	resp := Handle(context.Background(), req, Dependencies{ConfigOptions: config.Options{ProjectDir: filepath.Join(root, ".omg")}})

	if !resp.OK || resp.Action != protocol.ActionInjectPrompt {
		t.Fatalf("response = %#v", resp)
	}
	for _, want := range []string{
		"Tool policy: allowlist",
		"read, grep, edit",
		"Autonomy: cautious",
		"Ask for confirmation before risky, destructive, external, credential-gated, or production-impacting actions.",
		"Use when:",
		"Multi-file implementation work",
		"Avoid when:",
		"Pure research with no edits",
	} {
		if !strings.Contains(resp.Output, want) {
			t.Fatalf("output missing %q:\n%s", want, resp.Output)
		}
	}
}

func TestHandleOnToolBeforeReflectsGuardDecision(t *testing.T) {
	req := &Request{Version: protocol.Version, RequestID: "tool-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnToolBefore, Project: t.TempDir(), CWD: t.TempDir(), Payload: map[string]any{"tool": "write", "args": map[string]any{"path": "danger.txt"}}}
	blockRule := testGuardRule{
		id: "block-write",
		fn: func(input guard.GuardInput) (guard.GuardDecision, error) {
			if input.Tool != "write" || input.Args["path"] != "danger.txt" {
				t.Fatalf("guard input = %#v", input)
			}
			return guard.Decision(guard.GuardBlock, guard.SeverityDanger, "writes are blocked", "block-write"), nil
		},
	}

	resp := Handle(context.Background(), req, Dependencies{GuardRules: []guard.Rule{blockRule}})

	if !resp.OK || resp.Action != protocol.ActionBlock || !strings.Contains(resp.Output, "writes are blocked") {
		t.Fatalf("response = %#v", resp)
	}
}

func TestHandleOnErrorTransitionsStateToFailed(t *testing.T) {
	root := t.TempDir()
	store := state.NewStore(root, state.Options{})
	now := time.Date(2026, 5, 6, 1, 2, 3, 0, time.UTC)
	if _, err := store.Transition("session-1", state.TransitionInput{To: state.LifecycleRunning, Context: "execute", RequestID: "start", Now: now}); err != nil {
		t.Fatalf("seed running state: %v", err)
	}
	req := &Request{Version: protocol.Version, RequestID: "err-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnError, Project: root, CWD: root, SessionID: "session-1", Payload: map[string]any{"error_class": "provider_unavailable", "error_message": "provider down"}}

	resp := Handle(context.Background(), req, Dependencies{StateStore: store, Now: func() time.Time { return now.Add(time.Minute) }})

	if !resp.OK || resp.Action != protocol.ActionNone {
		t.Fatalf("response = %#v", resp)
	}
	record, err := store.Read("session-1")
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if record.Lifecycle != state.LifecycleFailed || record.RequestID != "err-1" || record.Error == nil || record.Error.Message != "provider down" {
		t.Fatalf("record = %#v", record)
	}
}

func writeConfig(t *testing.T, root, content string) {
	t.Helper()
	dir := filepath.Join(root, ".omg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir .omg: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

type testGuardRule struct {
	id string
	fn func(guard.GuardInput) (guard.GuardDecision, error)
}

func (r testGuardRule) ID() string { return r.id }
func (r testGuardRule) Evaluate(input guard.GuardInput) (guard.GuardDecision, error) {
	return r.fn(input)
}

type recordingSink struct {
	events []storage.Event
}

func (s *recordingSink) RecordEvent(_ context.Context, event storage.Event) error {
	s.events = append(s.events, event)
	return nil
}

type rejectingSink struct {
	events []storage.Event
	failOn storage.EventType
}

func (r *rejectingSink) RecordEvent(_ context.Context, event storage.Event) error {
	if event.Type == r.failOn {
		return fmt.Errorf("simulated storage rejection for %s", event.Type)
	}
	r.events = append(r.events, event)
	return nil
}

func TestHandleOnToolBeforeRecordsCoreDBEvent(t *testing.T) {
	root := t.TempDir()
	sink := &recordingSink{}
	req := &Request{Version: protocol.Version, RequestID: "tool-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnToolBefore, Project: root, CWD: root, SessionID: "agent-1", Payload: map[string]any{"run_id": "run-1", "work_id": "work-1", "tool": "write", "args": map[string]any{"path": "danger.txt"}}}
	blockRule := testGuardRule{id: "block-write", fn: func(input guard.GuardInput) (guard.GuardDecision, error) {
		return guard.Decision(guard.GuardBlock, guard.SeverityDanger, "writes are blocked", "block-write"), nil
	}}

	resp := Handle(context.Background(), req, Dependencies{StorageSink: sink, GuardRules: []guard.Rule{blockRule}})

	if !resp.OK || resp.Action != protocol.ActionBlock {
		t.Fatalf("response = %#v", resp)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Type != storage.EventToolCallBlocked || event.RunID != "run-1" || event.WorkID != "work-1" || event.AgentSessionID != "agent-1" || event.Status != "blocked" {
		t.Fatalf("event = %#v", event)
	}
	if event.Attributes["arg_path"] != "danger.txt" {
		t.Fatalf("event attributes = %#v", event.Attributes)
	}
}

func TestHandleOnErrorRecordsStateTransitionEvent(t *testing.T) {
	root := t.TempDir()
	store := state.NewStore(root, state.Options{})
	sink := &recordingSink{}
	now := time.Date(2026, 5, 15, 1, 2, 3, 0, time.UTC)
	if _, err := store.Transition("session-1", state.TransitionInput{To: state.LifecycleRunning, Context: "execute", RequestID: "start", Now: now}); err != nil {
		t.Fatalf("seed running state: %v", err)
	}
	req := &Request{Version: protocol.Version, RequestID: "err-1", Provider: protocol.ProviderDirect, Event: protocol.EventOnError, Project: root, CWD: root, SessionID: "session-1", Payload: map[string]any{"run_id": "run-1", "error_class": "provider_unavailable"}}

	resp := Handle(context.Background(), req, Dependencies{StateStore: store, StorageSink: sink, Now: func() time.Time { return now.Add(time.Minute) }})

	if !resp.OK || resp.Action != protocol.ActionNone {
		t.Fatalf("response = %#v", resp)
	}
	if len(sink.events) != 1 {
		t.Fatalf("events = %d, want 1", len(sink.events))
	}
	event := sink.events[0]
	if event.Type != storage.EventRunStatusChanged || event.RunID != "run-1" || event.Lifecycle != string(state.LifecycleFailed) || event.Timestamp != now.Add(time.Minute) {
		t.Fatalf("event = %#v", event)
	}
}
