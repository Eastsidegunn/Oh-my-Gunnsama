package pi

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %s: %v", path, err)
	}
	return bytes.TrimRight(data, "\n")
}

func TestParseLine_AgentStart(t *testing.T) {
	line := loadFixture(t, "events_agent_start.jsonl")
	evt, err := ParseLine(line, 42)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if evt.Kind != "agent_start" {
		t.Fatalf("Kind = %q, want %q", evt.Kind, "agent_start")
	}
	if evt.Sequence != 42 {
		t.Fatalf("Sequence = %d, want 42", evt.Sequence)
	}
	if !bytes.Equal(evt.Raw, line) {
		t.Fatalf("Raw = %q, want %q", evt.Raw, line)
	}
	want, _ := time.Parse(time.RFC3339Nano, "2026-05-19T10:30:00.000Z")
	if !evt.Timestamp.Equal(want) {
		t.Fatalf("Timestamp = %v, want %v", evt.Timestamp, want)
	}
}

func TestParseLine_ToolStart(t *testing.T) {
	line := loadFixture(t, "events_tool_start.jsonl")
	evt, err := ParseLine(line, 1)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if evt.Kind != "tool_execution_start" {
		t.Fatalf("Kind = %q, want tool_execution_start", evt.Kind)
	}
	if evt.ToolName != "bash" {
		t.Fatalf("ToolName = %q, want bash", evt.ToolName)
	}
	cmd, ok := evt.ToolArgs["command"]
	if !ok {
		t.Fatalf("ToolArgs missing 'command' key: %+v", evt.ToolArgs)
	}
	if cmd != "ls -la" {
		t.Fatalf("ToolArgs[command] = %v, want %q", cmd, "ls -la")
	}
}

func TestParseLine_ToolEnd(t *testing.T) {
	line := loadFixture(t, "events_tool_end.jsonl")
	evt, err := ParseLine(line, 2)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if evt.Kind != "tool_execution_end" {
		t.Fatalf("Kind = %q, want tool_execution_end", evt.Kind)
	}
	if evt.ToolName != "bash" {
		t.Fatalf("ToolName = %q, want bash", evt.ToolName)
	}
	exitCode, ok := evt.ToolArgs["exitCode"]
	if !ok {
		t.Fatalf("ToolArgs missing 'exitCode' key: %+v", evt.ToolArgs)
	}
	if got, want := exitCode, float64(0); got != want {
		t.Fatalf("ToolArgs[exitCode] = %v (%T), want %v (float64)", got, got, want)
	}
}

func TestParseLine_MessageEnd(t *testing.T) {
	line := loadFixture(t, "events_message_end.jsonl")
	evt, err := ParseLine(line, 3)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if evt.Kind != "message_end" {
		t.Fatalf("Kind = %q, want message_end", evt.Kind)
	}
	if !strings.Contains(evt.Message, "Hello world") {
		t.Fatalf("Message = %q, want substring %q", evt.Message, "Hello world")
	}
}

func TestParseLine_AgentEnd(t *testing.T) {
	line := loadFixture(t, "events_agent_end.jsonl")
	evt, err := ParseLine(line, 4)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if evt.Kind != "agent_end" {
		t.Fatalf("Kind = %q, want agent_end", evt.Kind)
	}
}

func TestParseLine_UnknownKindForwardCompat(t *testing.T) {
	line := []byte(`{"type":"future_kind","extra":"data"}`)
	evt, err := ParseLine(line, 5)
	if err != nil {
		t.Fatalf("ParseLine returned error for unknown kind: %v", err)
	}
	if evt.Kind != "future_kind" {
		t.Fatalf("Kind = %q, want future_kind", evt.Kind)
	}
}

func TestParseLine_SessionHeaderSkipped(t *testing.T) {
	line := []byte(`{"type":"session","version":3,"id":"abc","cwd":"/tmp"}`)
	_, err := ParseLine(line, 0)
	if !errors.Is(err, ErrSkipLine) {
		t.Fatalf("err = %v, want errors.Is ErrSkipLine", err)
	}
}

func TestParseLine_EmptyLine(t *testing.T) {
	cases := [][]byte{
		[]byte(""),
		[]byte("   \n"),
		[]byte("\t\r\n"),
	}
	for i, line := range cases {
		_, err := ParseLine(line, i)
		if !errors.Is(err, ErrSkipLine) {
			t.Fatalf("case %d: err = %v, want errors.Is ErrSkipLine", i, err)
		}
	}
}

func TestParseLine_MalformedJSON(t *testing.T) {
	line := []byte("not json{")
	_, err := ParseLine(line, 0)
	if err == nil {
		t.Fatalf("ParseLine returned nil error for malformed JSON")
	}
	if errors.Is(err, ErrSkipLine) {
		t.Fatalf("err = %v, should not be ErrSkipLine", err)
	}
}

func TestParseLine_MissingType(t *testing.T) {
	line := []byte(`{"foo":"bar"}`)
	_, err := ParseLine(line, 0)
	if err == nil {
		t.Fatalf("ParseLine returned nil error for event missing type")
	}
	if !strings.Contains(err.Error(), "missing type") {
		t.Fatalf("err = %v, want error containing 'missing type'", err)
	}
}

func TestParseLine_TimestampFallback(t *testing.T) {
	line := []byte(`{"type":"agent_start"}`)
	before := time.Now().UTC().Add(-time.Second)
	evt, err := ParseLine(line, 0)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	if evt.Timestamp.IsZero() {
		t.Fatalf("Timestamp is zero, want fallback to now")
	}
	if evt.Timestamp.Before(before) {
		t.Fatalf("Timestamp = %v, want >= %v", evt.Timestamp, before)
	}
}

func TestParseLine_RawPreserved(t *testing.T) {
	line := []byte(`{"type":"agent_start","timestamp":"2026-05-19T10:30:00.000Z"}`)
	original := append([]byte(nil), line...)
	evt, err := ParseLine(line, 0)
	if err != nil {
		t.Fatalf("ParseLine returned error: %v", err)
	}
	for i := range line {
		line[i] = 'X'
	}
	if !bytes.Equal(evt.Raw, original) {
		t.Fatalf("Raw was mutated: got %q, want %q", evt.Raw, original)
	}
}
