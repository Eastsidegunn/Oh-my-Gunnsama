package state

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLegalTransitionSucceedsAndSeparatesContext(t *testing.T) {
	store := NewStore(t.TempDir(), Options{})
	now := time.Date(2026, 5, 6, 1, 2, 3, 0, time.UTC)

	record, err := store.Transition("session-1", TransitionInput{
		To:        LifecycleRunning,
		Context:   "execute",
		OwnerID:   "daemon-1",
		RequestID: "req-1",
		Now:       now,
	})
	if err != nil {
		t.Fatalf("Transition returned error: %v", err)
	}
	if record.Lifecycle != LifecycleRunning || record.Context != "execute" {
		t.Fatalf("record lifecycle/context mismatch: %#v", record)
	}

	read, err := store.Read("session-1")
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if read.Lifecycle != LifecycleRunning || read.Context != "execute" || read.RequestID != "req-1" {
		t.Fatalf("persisted record mismatch: %#v", read)
	}
}

func TestIllegalTransitionReturnsError(t *testing.T) {
	store := NewStore(t.TempDir(), Options{})
	_, err := store.Transition("session-1", TransitionInput{To: LifecycleFinished, Now: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "illegal transition") {
		t.Fatalf("error = %v, want illegal transition", err)
	}
}

func TestStaleHeartbeatTransitionsRunningToBlockedNotFailed(t *testing.T) {
	store := NewStore(t.TempDir(), Options{HeartbeatTimeout: 120 * time.Second})
	now := time.Date(2026, 5, 6, 1, 2, 3, 0, time.UTC)
	_, err := store.Transition("session-1", TransitionInput{To: LifecycleRunning, Context: "verify", OwnerID: "daemon-1", Now: now})
	if err != nil {
		t.Fatalf("Transition running returned error: %v", err)
	}

	record, stale, err := store.MarkStaleHeartbeats(now.Add(121 * time.Second))
	if err != nil {
		t.Fatalf("MarkStaleHeartbeats returned error: %v", err)
	}
	if !stale {
		t.Fatalf("expected stale heartbeat to be detected")
	}
	if record.Lifecycle != LifecycleBlocked || record.BlockedReason != BlockedReasonStaleOwner {
		t.Fatalf("stale record = %#v, want blocked stale_owner", record)
	}
	if record.Lifecycle == LifecycleFailed {
		t.Fatalf("stale heartbeat must not become failed")
	}
}

func TestAtomicWriteFailureLeavesPreviousRecordIntact(t *testing.T) {
	store := NewStore(t.TempDir(), Options{})
	initial, err := store.Transition("session-1", TransitionInput{To: LifecycleRunning, Context: "execute", Now: time.Now()})
	if err != nil {
		t.Fatalf("initial Transition returned error: %v", err)
	}

	store.failBeforeRename = errors.New("simulated crash before rename")
	_, err = store.Transition("session-1", TransitionInput{To: LifecycleFinished, Now: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "simulated crash") {
		t.Fatalf("error = %v, want simulated crash", err)
	}

	read, err := store.Read("session-1")
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if read.Lifecycle != initial.Lifecycle || read.Context != initial.Context {
		t.Fatalf("atomic write failure changed persisted record: %#v", read)
	}

	matches, err := filepath.Glob(filepath.Join(store.sessionDir(), "*.tmp.*"))
	if err != nil {
		t.Fatalf("glob tmp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not cleaned up: %#v", matches)
	}
}

func TestCancelledIsTerminal(t *testing.T) {
	store := NewStore(t.TempDir(), Options{})
	_, err := store.Transition("session-1", TransitionInput{To: LifecycleRunning, Now: time.Now()})
	if err != nil {
		t.Fatalf("running Transition returned error: %v", err)
	}
	_, err = store.Transition("session-1", TransitionInput{To: LifecycleCancelled, Now: time.Now()})
	if err != nil {
		t.Fatalf("cancel Transition returned error: %v", err)
	}
	_, err = store.Transition("session-1", TransitionInput{To: LifecycleRunning, Now: time.Now()})
	if err == nil || !strings.Contains(err.Error(), "terminal") {
		t.Fatalf("error = %v, want terminal transition error", err)
	}
}

func TestSessionStateWinsOverProjectRootState(t *testing.T) {
	store := NewStore(t.TempDir(), Options{})
	rootRecord := StateRecord{Lifecycle: LifecycleRunning, Context: "project", UpdatedAt: time.Now()}
	if err := store.WriteRoot(rootRecord); err != nil {
		t.Fatalf("WriteRoot returned error: %v", err)
	}
	_, err := store.Transition("session-1", TransitionInput{To: LifecycleRunning, Context: "session", Now: time.Now()})
	if err != nil {
		t.Fatalf("Transition returned error: %v", err)
	}
	read, err := store.ReadEffective("session-1")
	if err != nil {
		t.Fatalf("ReadEffective returned error: %v", err)
	}
	if read.Context != "session" {
		t.Fatalf("effective context = %q, want session", read.Context)
	}
}

func TestAtomicWriteCreatesStateDirectory(t *testing.T) {
	root := t.TempDir()
	store := NewStore(root, Options{})
	if _, err := os.Stat(filepath.Join(root, ".omg", "state")); !os.IsNotExist(err) {
		t.Fatalf("state dir should not exist before write, stat err=%v", err)
	}
	_, err := store.Transition("session-1", TransitionInput{To: LifecycleRunning, Now: time.Now()})
	if err != nil {
		t.Fatalf("Transition returned error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".omg", "state", "sessions", "session-1")); err != nil {
		t.Fatalf("state session dir was not created: %v", err)
	}
}
