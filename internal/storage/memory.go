package storage

import (
	"context"
	"sync"
)

// MemorySink is an in-memory implementation of Sink for tests. It captures
// all RecordEvent calls and exposes them via Snapshot/Count. Thread-safe.
type MemorySink struct {
	mu     sync.Mutex
	events []Event
}

// NewMemorySink returns a new MemorySink with an empty event list.
func NewMemorySink() *MemorySink { return &MemorySink{} }

// RecordEvent appends the event to the in-memory log. Always returns nil.
func (m *MemorySink) RecordEvent(ctx context.Context, e Event) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, e)
	return nil
}

// Snapshot returns a copy of all recorded events. Mutating the returned
// slice does not affect the sink's internal state.
func (m *MemorySink) Snapshot() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, len(m.events))
	copy(out, m.events)
	return out
}

// Count returns the number of events with the given type.
func (m *MemorySink) Count(t EventType) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, e := range m.events {
		if e.Type == t {
			n++
		}
	}
	return n
}

// Reset clears all recorded events.
func (m *MemorySink) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = nil
}
