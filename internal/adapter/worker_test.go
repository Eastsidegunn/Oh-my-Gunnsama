package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

type fakeWorker struct{}

func (f *fakeWorker) Spawn(ctx context.Context, spec WorkerSpec) error { return nil }
func (f *fakeWorker) Events() <-chan WorkerEvent                       { return nil }
func (f *fakeWorker) Wait(ctx context.Context) (WorkerResult, error)   { return WorkerResult{}, nil }
func (f *fakeWorker) Abort(ctx context.Context) error                  { return nil }

func TestWorker_InterfaceSatisfaction(t *testing.T) {
	var _ Worker = (*fakeWorker)(nil)
}

func TestWorkerEvent_JSONRoundtrip(t *testing.T) {
	original := WorkerEvent{
		Kind:      "tool_use",
		Timestamp: time.Date(2026, 5, 19, 12, 0, 0, 0, time.UTC),
		Message:   "running tool",
		ToolName:  "read_file",
		ToolArgs:  map[string]any{"path": "/tmp/x"},
		Raw:       []byte("should-not-appear"),
		Sequence:  42,
	}

	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if bytes.Contains(encoded, []byte("should-not-appear")) {
		t.Fatalf("Raw field leaked into JSON output: %s", encoded)
	}
	if bytes.Contains(encoded, []byte(`"Raw"`)) || bytes.Contains(encoded, []byte(`"raw"`)) {
		t.Fatalf("Raw field key present in JSON: %s", encoded)
	}

	var decoded WorkerEvent
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	expected := original
	expected.Raw = nil
	if !reflect.DeepEqual(expected, decoded) {
		t.Fatalf("roundtrip mismatch:\n want: %#v\n got:  %#v", expected, decoded)
	}
}

func TestWorkerSpec_ZeroValueOK(t *testing.T) {
	var spec WorkerSpec
	if spec.Goal != "" || spec.ProjectDir != "" || spec.AllowTools != nil || spec.Env != nil {
		t.Fatalf("zero-value WorkerSpec not zero: %#v", spec)
	}

	encoded, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal zero spec: %v", err)
	}
	var decoded WorkerSpec
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal zero spec: %v", err)
	}
}
