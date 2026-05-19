package pi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"oh-my-gunnsama/internal/adapter"
)

// ErrSkipLine signals that the line should be parsed without surfacing
// (e.g., empty lines, pi session-header preamble).
var ErrSkipLine = errors.New("skip line")

// ParseLine converts one JSONL line from pi --mode json into a WorkerEvent.
// Returns ErrSkipLine for whitespace-only lines and the session-header preamble.
// Unknown event kinds are not rejected; they pass through with Kind = the raw type.
func ParseLine(line []byte, sequence int) (adapter.WorkerEvent, error) {
	trimmed := bytes.TrimRight(line, "\r")
	if len(bytes.TrimSpace(trimmed)) == 0 {
		return adapter.WorkerEvent{}, ErrSkipLine
	}
	var probe struct {
		Type       string         `json:"type"`
		Timestamp  string         `json:"timestamp,omitempty"`
		ToolCallID string         `json:"toolCallId,omitempty"`
		ToolName   string         `json:"toolName,omitempty"`
		Args       map[string]any `json:"args,omitempty"`
		Result     map[string]any `json:"result,omitempty"`
		Message    map[string]any `json:"message,omitempty"`
	}
	if err := json.Unmarshal(trimmed, &probe); err != nil {
		return adapter.WorkerEvent{}, fmt.Errorf("parse pi event: %w", err)
	}
	if probe.Type == "" {
		return adapter.WorkerEvent{}, fmt.Errorf("pi event missing type")
	}
	// session header is a one-time preamble, not a worker event
	if probe.Type == "session" {
		return adapter.WorkerEvent{}, ErrSkipLine
	}
	evt := adapter.WorkerEvent{
		Kind:     probe.Type,
		Sequence: sequence,
		Raw:      append([]byte(nil), trimmed...),
	}
	if probe.Timestamp != "" {
		if ts, err := time.Parse(time.RFC3339Nano, probe.Timestamp); err == nil {
			evt.Timestamp = ts
		}
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now().UTC()
	}
	switch probe.Type {
	case "tool_execution_start", "tool_execution_update":
		evt.ToolName = probe.ToolName
		evt.ToolArgs = probe.Args
	case "tool_execution_end":
		evt.ToolName = probe.ToolName
		evt.ToolArgs = probe.Result
	case "message_start", "message_update", "message_end":
		if probe.Message != nil {
			if content, ok := probe.Message["content"]; ok {
				evt.Message = fmt.Sprintf("%v", content)
			}
		}
	}
	return evt, nil
}
