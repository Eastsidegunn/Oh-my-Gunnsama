package adapter

import (
	"context"
	"sync"
)

// MockWorker is a test double implementing Worker. It emits a predefined
// sequence of events when Spawn is called, records observability data
// (call counts, last spec), and returns configured results from Wait.
//
// Public so other packages can construct it in tests.
type MockWorker struct {
	// Configurable behavior (set before Spawn):
	Events_  []WorkerEvent
	Result_  WorkerResult
	WaitErr  error
	SpawnErr error

	// Observability (read after Spawn/Abort):
	mu          sync.Mutex
	SpawnCalls  int
	SpawnedSpec WorkerSpec
	AbortCalls  int

	// Internal channels:
	ch   chan WorkerEvent
	done chan struct{}
}

// Spawn starts the worker. If SpawnErr is configured, returns it after
// recording the spec/call but without opening any channels.
func (m *MockWorker) Spawn(ctx context.Context, spec WorkerSpec) error {
	m.mu.Lock()
	m.SpawnCalls++
	m.SpawnedSpec = spec
	spawnErr := m.SpawnErr
	events := append([]WorkerEvent(nil), m.Events_...)
	m.mu.Unlock()

	if spawnErr != nil {
		return spawnErr
	}

	ch := make(chan WorkerEvent, len(events))
	done := make(chan struct{})
	for _, e := range events {
		ch <- e
	}
	close(ch)
	close(done)

	m.mu.Lock()
	m.ch = ch
	m.done = done
	m.mu.Unlock()
	return nil
}

// Events returns the channel of pending events. nil if SpawnErr was set
// or Spawn was never called.
func (m *MockWorker) Events() <-chan WorkerEvent {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ch
}

// Wait returns the configured Result_ and WaitErr.
func (m *MockWorker) Wait(ctx context.Context) (WorkerResult, error) {
	m.mu.Lock()
	result := m.Result_
	err := m.WaitErr
	m.mu.Unlock()
	return result, err
}

// Abort increments AbortCalls counter. Always returns nil.
func (m *MockWorker) Abort(ctx context.Context) error {
	m.mu.Lock()
	m.AbortCalls++
	m.mu.Unlock()
	return nil
}
