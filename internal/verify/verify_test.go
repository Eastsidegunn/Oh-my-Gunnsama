package verify

import (
	"context"
	"strings"
	"testing"
	"time"

	"oh-my-gunnsama/internal/storage"
)

func TestRunPlanCollectsCommandEvidence(t *testing.T) {
	runner := &fakeRunner{results: map[string]CommandResult{
		"go-test": {ExitCode: 0, Stdout: "ok", Stderr: "", Duration: 10 * time.Millisecond},
	}}
	result, err := RunPlan(context.Background(), VerificationPlan{Checks: []VerificationCheck{{
		ID: "go-test", Name: "Go tests", Command: []string{"go", "test", "./..."}, Timeout: time.Second, Claim: "tests pass", Required: true, EvidenceKind: EvidenceCommand,
	}}}, Options{Runner: runner})
	if err != nil {
		t.Fatalf("RunPlan returned error: %v", err)
	}
	if !result.Passed || len(result.Evidence) != 1 {
		t.Fatalf("result = %#v", result)
	}
	e := result.Evidence[0]
	if e.CheckID != "go-test" || !e.Passed || e.ExitCode != 0 || e.Stdout != "ok" || e.Duration != 10*time.Millisecond {
		t.Fatalf("evidence mismatch: %#v", e)
	}
}

func TestRequiredFailureFailsOverall(t *testing.T) {
	runner := &fakeRunner{results: map[string]CommandResult{
		"go-test": {ExitCode: 1, Stdout: "", Stderr: "fail", Duration: time.Millisecond},
	}}
	result, err := RunPlan(context.Background(), VerificationPlan{Checks: []VerificationCheck{{
		ID: "go-test", Command: []string{"go", "test"}, Timeout: time.Second, Required: true, EvidenceKind: EvidenceCommand,
	}}}, Options{Runner: runner})
	if err != nil {
		t.Fatalf("RunPlan returned error: %v", err)
	}
	if result.Passed {
		t.Fatalf("required failure should fail overall: %#v", result)
	}
}

func TestNoEvidenceCannotPass(t *testing.T) {
	result, err := RunPlan(context.Background(), VerificationPlan{}, Options{Runner: &fakeRunner{}})
	if err != nil {
		t.Fatalf("RunPlan returned error: %v", err)
	}
	if result.Passed || !strings.Contains(result.Summary, "no evidence") {
		t.Fatalf("result = %#v, want no-evidence failure", result)
	}
}

func TestManualEvidenceWithoutReasonRejected(t *testing.T) {
	_, err := RunPlan(context.Background(), VerificationPlan{Checks: []VerificationCheck{{
		ID: "manual", EvidenceKind: EvidenceManual, Required: true,
	}}}, Options{Runner: &fakeRunner{}})
	if err == nil || !strings.Contains(err.Error(), "manual evidence reason") {
		t.Fatalf("error = %v, want manual reason error", err)
	}
}

func TestCommandTimeoutProducesFailedEvidence(t *testing.T) {
	runner := &fakeRunner{errByID: map[string]error{"slow": context.DeadlineExceeded}}
	result, err := RunPlan(context.Background(), VerificationPlan{Checks: []VerificationCheck{{
		ID: "slow", Command: []string{"sleep", "10"}, Timeout: time.Millisecond, Required: true, EvidenceKind: EvidenceCommand,
	}}}, Options{Runner: runner})
	if err != nil {
		t.Fatalf("RunPlan returned error: %v", err)
	}
	if result.Passed {
		t.Fatalf("timeout should fail result: %#v", result)
	}
	if len(result.Evidence) != 1 || result.Evidence[0].Passed || !strings.Contains(result.Evidence[0].Stderr, "deadline") {
		t.Fatalf("timeout evidence mismatch: %#v", result.Evidence)
	}
}

func TestArtifactEvidenceRequiresArtifactPath(t *testing.T) {
	_, err := RunPlan(context.Background(), VerificationPlan{Checks: []VerificationCheck{{
		ID: "artifact", EvidenceKind: EvidenceArtifact, Required: true,
	}}}, Options{Runner: &fakeRunner{}})
	if err == nil || !strings.Contains(err.Error(), "artifact") {
		t.Fatalf("error = %v, want artifact path error", err)
	}
}

type fakeRunner struct {
	results map[string]CommandResult
	errByID map[string]error
	calls   int
}

func (f *fakeRunner) Run(ctx context.Context, check VerificationCheck) (CommandResult, error) {
	f.calls++
	if err := f.errByID[check.ID]; err != nil {
		return CommandResult{}, err
	}
	return f.results[check.ID], nil
}

type verifyRecordingSink struct {
	events []storage.Event
}

func (s *verifyRecordingSink) RecordEvent(_ context.Context, event storage.Event) error {
	s.events = append(s.events, event)
	return nil
}

func TestRunPlanRecordsCoreDBVerificationEvents(t *testing.T) {
	sink := &verifyRecordingSink{}
	result, err := RunPlan(context.Background(), VerificationPlan{Checks: []VerificationCheck{{
		ID:           "unit",
		Name:         "unit tests",
		Command:      []string{"go", "test", "./internal/storage"},
		Required:     true,
		EvidenceKind: EvidenceCommand,
	}}}, Options{
		Runner:         &fakeRunner{results: map[string]CommandResult{"unit": {ExitCode: 0, Stdout: "ok"}}},
		StorageSink:    sink,
		RunID:          "run-1",
		WorkID:         "work-1",
		VerificationID: "verify-1",
		ActorID:        "verifier-1",
	})

	if err != nil {
		t.Fatalf("RunPlan returned error: %v", err)
	}
	if !result.Passed {
		t.Fatalf("result = %#v", result)
	}
	if len(sink.events) != 2 {
		t.Fatalf("events = %d, want verification + evidence", len(sink.events))
	}
	if sink.events[0].Type != storage.EventVerificationPassed || sink.events[0].RunID != "run-1" || sink.events[0].WorkID != "work-1" {
		t.Fatalf("verification event = %#v", sink.events[0])
	}
	if sink.events[1].Type != storage.EventEvidenceCreated || sink.events[1].Attributes["check_id"] != "unit" {
		t.Fatalf("evidence event = %#v", sink.events[1])
	}
}
