package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestOpenStoreMigratesDBMLTablesInRepoLocalOMG(t *testing.T) {
	root := t.TempDir()
	store, err := OpenStore(context.Background(), root, Options{})
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	defer store.Shutdown(context.Background())
	if store.Path() != filepath.Join(root, ".omg", "omg.db") {
		t.Fatalf("path = %q", store.Path())
	}
	for _, table := range []string{"repos", "runs", "work_items", "agent_sessions", "context_packets", "permission_policies", "tool_calls", "evidence", "verification_results", "failure_diagnoses", "events", "messages", "approval_requests", "memory_candidates", "reports"} {
		exists, err := store.HasTable(context.Background(), table)
		if err != nil {
			t.Fatalf("HasTable(%s): %v", table, err)
		}
		if !exists {
			t.Fatalf("table %s does not exist", table)
		}
	}
}

func TestRecordEventWritesRunWorkToolAndEventRows(t *testing.T) {
	store, err := OpenStore(context.Background(), t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	defer store.Shutdown(context.Background())
	now := time.Date(2026, 5, 16, 1, 2, 3, 0, time.UTC)
	err = store.RecordEvent(context.Background(), Event{
		Type:           EventToolCallBlocked,
		RunID:          "run-1",
		WorkID:         "work-1",
		AgentSessionID: "agent-1",
		ToolCallID:     "tool-1",
		ProjectRoot:    "/repo",
		Status:         "blocked",
		ActorType:      "tool",
		Attributes:     map[string]string{"tool": "write", "blocked_reason": "policy"},
		Timestamp:      now,
	})
	if err != nil {
		t.Fatalf("RecordEvent returned error: %v", err)
	}
	for _, tc := range []struct {
		table string
		want  int
	}{{"repos", 1}, {"runs", 1}, {"work_items", 1}, {"agent_sessions", 1}, {"tool_calls", 1}, {"events", 1}} {
		got, err := store.Count(context.Background(), tc.table)
		if err != nil {
			t.Fatalf("Count(%s): %v", tc.table, err)
		}
		if got != tc.want {
			t.Fatalf("Count(%s) = %d, want %d", tc.table, got, tc.want)
		}
	}
}

func TestRecordVerificationAndEvidenceRows(t *testing.T) {
	store, err := OpenStore(context.Background(), t.TempDir(), Options{})
	if err != nil {
		t.Fatalf("OpenStore returned error: %v", err)
	}
	defer store.Shutdown(context.Background())
	base := Event{RunID: "run-1", WorkID: "work-1", ProjectRoot: "/repo", Timestamp: time.Now().UTC()}
	if err := store.RecordEvent(context.Background(), Event{Type: EventVerificationPassed, VerificationID: "verify-1", Status: "passed", Attributes: map[string]string{"summary": "ok"}, RunID: base.RunID, WorkID: base.WorkID, ProjectRoot: base.ProjectRoot, Timestamp: base.Timestamp}); err != nil {
		t.Fatalf("verification event: %v", err)
	}
	if err := store.RecordEvent(context.Background(), Event{Type: EventEvidenceCreated, EvidenceID: "evidence-1", VerificationID: "verify-1", Attributes: map[string]string{"kind": "test_result", "claim": "tests pass"}, RunID: base.RunID, WorkID: base.WorkID, ProjectRoot: base.ProjectRoot, Timestamp: base.Timestamp}); err != nil {
		t.Fatalf("evidence event: %v", err)
	}
	for _, table := range []string{"verification_results", "evidence", "events"} {
		got, err := store.Count(context.Background(), table)
		if err != nil {
			t.Fatalf("Count(%s): %v", table, err)
		}
		if got == 0 {
			t.Fatalf("Count(%s) = 0", table)
		}
	}
}
