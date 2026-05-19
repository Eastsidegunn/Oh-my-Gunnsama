package protocol

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestReadRequestDecodesValidEnvelopeAndPreservesOpaqueFields(t *testing.T) {
	input := `{
		"version":1,
		"request_id":"req-123",
		"provider":"codex",
		"event":"on-submit",
		"project":"/repo",
		"cwd":"/repo/subdir",
		"session_id":"sess-1",
		"timeout_ms":500,
		"capabilities":{"supports_exec_hook":true,"nested":{"x":1}},
		"payload":{"input":"hello","unknown":{"kept":true}},
		"future_top_level":"ignored"
	}`

	req, perr := ReadRequest(strings.NewReader(input))
	if perr != nil {
		t.Fatalf("ReadRequest returned error: %v", perr)
	}
	if req.RequestID != "req-123" || req.Provider != ProviderCodex || req.Event != EventOnSubmit {
		t.Fatalf("decoded stable fields incorrectly: %#v", req)
	}
	if req.Capabilities["supports_exec_hook"] != true {
		t.Fatalf("capabilities were not preserved: %#v", req.Capabilities)
	}
	payload, ok := req.Payload["unknown"].(map[string]any)
	if !ok || payload["kept"] != true {
		t.Fatalf("payload was not preserved: %#v", req.Payload)
	}
}

func TestWriteResponseEncodesSuccessEnvelopeWithNewline(t *testing.T) {
	req := &Request{RequestID: "req-123"}
	resp := OKResponse(req, ActionInjectPrompt, "system prompt", []string{"careful"})

	var buf bytes.Buffer
	if err := WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse returned error: %v", err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Fatalf("response is not line-delimited: %q", buf.String())
	}

	var decoded Response
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &decoded); err != nil {
		t.Fatalf("encoded response is invalid JSON: %v", err)
	}
	if !decoded.OK || decoded.RequestID != "req-123" || decoded.Action != ActionInjectPrompt || decoded.Output != "system prompt" {
		t.Fatalf("encoded response mismatch: %#v", decoded)
	}
	if decoded.Error != nil {
		t.Fatalf("success response should have null error: %#v", decoded.Error)
	}
}

func TestStructuredErrorResponse(t *testing.T) {
	req := &Request{RequestID: "req-structured"}
	perr := NewProtocolError(ErrorTimeout, "daemon timed out", true)
	resp := ErrorResponse(req, ActionWarn, perr, nil)

	var buf bytes.Buffer
	if err := WriteResponse(&buf, resp); err != nil {
		t.Fatalf("WriteResponse returned error: %v", err)
	}

	var decoded Response
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &decoded); err != nil {
		t.Fatalf("encoded error response is invalid JSON: %v", err)
	}
	if decoded.OK {
		t.Fatalf("error response must not be ok: %#v", decoded)
	}
	if decoded.Error == nil {
		t.Fatalf("error response must include structured error")
	}
	if decoded.Error.Class != ErrorTimeout || decoded.Error.Message != "daemon timed out" || !decoded.Error.Retryable {
		t.Fatalf("structured error mismatch: %#v", decoded.Error)
	}
	if len(decoded.Warnings) != 0 {
		t.Fatalf("nil warnings should encode as empty array: %#v", decoded.Warnings)
	}
}

func TestRequestIDEchoesWhenValidationFailsAfterDecode(t *testing.T) {
	input := `{
		"version":1,
		"request_id":"req-bad",
		"provider":"codex",
		"event":"on-submit",
		"project":"relative/path",
		"cwd":"/repo",
		"payload":{}
	}`

	req, perr := ReadRequest(strings.NewReader(input))
	if perr == nil {
		t.Fatalf("expected validation error")
	}
	if req == nil || req.RequestID != "req-bad" {
		t.Fatalf("decoded request_id should be available for echo: %#v", req)
	}
	resp := ErrorResponse(req, ActionWarn, perr, nil)
	if resp.RequestID != "req-bad" {
		t.Fatalf("request_id was not echoed: %#v", resp)
	}
	if resp.Error == nil || resp.Error.Class != ErrorInvalidConfig {
		t.Fatalf("expected structured invalid_config error: %#v", resp.Error)
	}
}

func TestMissingRequiredTopLevelFieldReturnsInvalidConfig(t *testing.T) {
	input := `{"version":1,"request_id":"req-missing","event":"on-submit","project":"/repo"}`

	req, perr := ReadRequest(strings.NewReader(input))
	if perr == nil {
		t.Fatalf("expected missing provider to fail")
	}
	if req == nil || req.RequestID != "req-missing" {
		t.Fatalf("request_id should still be decoded: %#v", req)
	}
	if perr.Class != ErrorInvalidConfig || perr.Retryable {
		t.Fatalf("unexpected protocol error: %#v", perr)
	}
}

func TestValidateRequest_OnRunAccepted(t *testing.T) {
	req := &Request{
		Version:   1,
		RequestID: "test",
		Provider:  ProviderDirect,
		Event:     EventOnRun,
	}
	if err := ValidateRequest(req); err != nil {
		t.Fatalf("ValidateRequest returned error for valid on-run: %v", err)
	}
}

func TestValidateRequest_UnknownEventRejected(t *testing.T) {
	req := &Request{
		Version:   1,
		RequestID: "x",
		Provider:  ProviderDirect,
		Event:     Event("on-nonsense"),
	}
	err := ValidateRequest(req)
	if err == nil {
		t.Fatalf("ValidateRequest returned nil for unknown event")
	}
	if err.Class != ErrorInvalidConfig {
		t.Fatalf("err.Class = %q, want %q", err.Class, ErrorInvalidConfig)
	}
	if !strings.Contains(strings.ToLower(err.Message), "event") {
		t.Fatalf("err.Message = %q, expected to contain 'event'", err.Message)
	}
}

func TestEventOnRun_RoundTripJSON(t *testing.T) {
	req := Request{
		Version:   1,
		RequestID: "r",
		Provider:  ProviderDirect,
		Event:     EventOnRun,
		Payload:   map[string]any{"goal": "create hello.txt"},
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got Request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got.Event != EventOnRun {
		t.Fatalf("Event = %q, want %q", got.Event, EventOnRun)
	}
	if goal, ok := got.Payload["goal"].(string); !ok || goal != "create hello.txt" {
		t.Fatalf("Payload[goal] = %v, want 'create hello.txt'", got.Payload["goal"])
	}
	_ = bytes.NewReader(data)
}
