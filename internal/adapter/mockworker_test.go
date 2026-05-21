package adapter

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestMockWorker_SpawnEmitsEventsThenCloses(t *testing.T) {
	w := &MockWorker{
		Events_: []WorkerEvent{
			{Kind: "a", Sequence: 1},
			{Kind: "b", Sequence: 2},
			{Kind: "c", Sequence: 3},
		},
	}
	if err := w.Spawn(context.Background(), WorkerSpec{}); err != nil {
		t.Fatalf("Spawn error: %v", err)
	}
	var got []string
	for e := range w.Events() {
		got = append(got, e.Kind)
	}
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMockWorker_WaitReturnsConfiguredResult(t *testing.T) {
	want := WorkerResult{ExitCode: 42, Success: true, Reason: "ok"}
	wantErr := errors.New("test-err")
	w := &MockWorker{Result_: want, WaitErr: wantErr}
	got, err := w.Wait(context.Background())
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got err %v, want %v", err, wantErr)
	}
}

func TestMockWorker_SpawnError_PreventsEvents(t *testing.T) {
	spawnErr := errors.New("boom")
	w := &MockWorker{SpawnErr: spawnErr, Events_: []WorkerEvent{{Kind: "x"}}}
	if err := w.Spawn(context.Background(), WorkerSpec{}); !errors.Is(err, spawnErr) {
		t.Fatalf("got err %v, want %v", err, spawnErr)
	}
	if w.Events() != nil {
		t.Errorf("Events() should be nil when SpawnErr set, got channel")
	}
	if w.SpawnCalls != 1 {
		t.Errorf("SpawnCalls=%d, want 1", w.SpawnCalls)
	}
}

func TestMockWorker_RecordsSpawnedSpec(t *testing.T) {
	spec := WorkerSpec{
		Goal:       "goal-x",
		ProjectDir: "/tmp/p",
		AllowTools: []string{"read", "bash"},
		Env:        map[string]string{"K": "V"},
	}
	w := &MockWorker{}
	if err := w.Spawn(context.Background(), spec); err != nil {
		t.Fatalf("Spawn error: %v", err)
	}
	if !reflect.DeepEqual(w.SpawnedSpec, spec) {
		t.Errorf("got %+v, want %+v", w.SpawnedSpec, spec)
	}
}

func TestMockWorker_AbortIncrementsCounter(t *testing.T) {
	w := &MockWorker{}
	for i := 0; i < 3; i++ {
		if err := w.Abort(context.Background()); err != nil {
			t.Fatalf("Abort error: %v", err)
		}
	}
	if w.AbortCalls != 3 {
		t.Errorf("AbortCalls=%d, want 3", w.AbortCalls)
	}
}

func TestMockWorker_InterfaceSatisfaction(t *testing.T) {
	var _ Worker = (*MockWorker)(nil)
}

func TestMockWorker_ConcurrentAbort_NoRace(t *testing.T) {
	w := &MockWorker{}
	var wg sync.WaitGroup
	const n = 50
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			_ = w.Abort(context.Background())
		}()
	}
	wg.Wait()
	if w.AbortCalls != n {
		t.Errorf("AbortCalls=%d, want %d", w.AbortCalls, n)
	}
}
