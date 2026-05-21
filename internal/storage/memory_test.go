package storage

import (
	"context"
	"sync"
	"testing"
)

func TestMemorySink_RecordEventStoresEvent(t *testing.T) {
	s := NewMemorySink()
	s.RecordEvent(context.Background(), Event{ID: "a", Type: EventRunCreated})
	s.RecordEvent(context.Background(), Event{ID: "b", Type: EventAgentSpawned})
	got := s.Snapshot()
	if len(got) != 2 {
		t.Fatalf("len=%d, want 2", len(got))
	}
	if got[0].ID != "a" || got[1].ID != "b" {
		t.Errorf("order wrong: %v", got)
	}
}

func TestMemorySink_SnapshotReturnsCopy(t *testing.T) {
	s := NewMemorySink()
	s.RecordEvent(context.Background(), Event{ID: "original"})
	snap1 := s.Snapshot()
	snap1[0].ID = "modified"
	snap2 := s.Snapshot()
	if snap2[0].ID != "original" {
		t.Errorf("Snapshot should return copy, got mutation: %s", snap2[0].ID)
	}
}

func TestMemorySink_CountByType(t *testing.T) {
	s := NewMemorySink()
	s.RecordEvent(context.Background(), Event{Type: EventRunCreated})
	s.RecordEvent(context.Background(), Event{Type: EventRunCreated})
	s.RecordEvent(context.Background(), Event{Type: EventAgentSpawned})

	if n := s.Count(EventRunCreated); n != 2 {
		t.Errorf("Count(RunCreated)=%d, want 2", n)
	}
	if n := s.Count(EventAgentSpawned); n != 1 {
		t.Errorf("Count(AgentSpawned)=%d, want 1", n)
	}
	if n := s.Count(EventWorkStarted); n != 0 {
		t.Errorf("Count(WorkStarted)=%d, want 0", n)
	}
}

func TestMemorySink_Reset(t *testing.T) {
	s := NewMemorySink()
	for i := 0; i < 5; i++ {
		s.RecordEvent(context.Background(), Event{Type: EventRunCreated})
	}
	s.Reset()
	if got := s.Snapshot(); len(got) != 0 {
		t.Errorf("after Reset, len=%d, want 0", len(got))
	}
}

func TestMemorySink_ConcurrentRecord_NoRace(t *testing.T) {
	s := NewMemorySink()
	var wg sync.WaitGroup
	const goroutines, perGoroutine = 100, 10
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				s.RecordEvent(context.Background(), Event{Type: EventRunCreated})
			}
		}()
	}
	wg.Wait()
	if got := len(s.Snapshot()); got != goroutines*perGoroutine {
		t.Errorf("total events=%d, want %d", got, goroutines*perGoroutine)
	}
}

func TestMemorySink_ImplementsSink(t *testing.T) {
	var _ Sink = (*MemorySink)(nil)
}
