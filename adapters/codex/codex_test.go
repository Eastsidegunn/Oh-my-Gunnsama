package codex

import (
	"encoding/json"
	"testing"
	"time"

	"oh-my-gunnsama/internal/protocol"
)

func TestTranslateEventConvertsCodexPayloadToEnvelope(t *testing.T) {
	adapter := New()
	raw := []byte(`{
		"request_id":"codex-1",
		"event":"UserPromptSubmit",
		"input":"hello",
		"project":"/repo",
		"cwd":"/repo/work",
		"timeout_ms":300
	}`)

	req, err := adapter.TranslateEvent(raw)
	if err != nil {
		t.Fatalf("TranslateEvent returned error: %v", err)
	}
	if req.Version != protocol.Version || req.Provider != protocol.ProviderCodex || req.Event != protocol.EventOnSubmit {
		t.Fatalf("bad envelope: %#v", req)
	}
	if req.RequestID != "codex-1" || req.CWD != "/repo/work" || req.TimeoutMS != 300 {
		t.Fatalf("bad stable fields: %#v", req)
	}
	if req.Payload["input"] != "hello" {
		t.Fatalf("payload input not preserved: %#v", req.Payload)
	}
}

func TestTranslateResponseAppliesBlockToCodexHostFormat(t *testing.T) {
	adapter := New()
	out, err := adapter.TranslateResponse(protocol.Response{OK: false, RequestID: "r", Action: protocol.ActionBlock, Output: "", Error: &protocol.ResponseError{Class: protocol.ErrorPolicyBlocked, Message: "blocked"}})
	if err != nil {
		t.Fatalf("TranslateResponse returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if decoded["action"] != "block" || decoded["message"] != "blocked" {
		t.Fatalf("bad host output: %#v", decoded)
	}
}

func TestCodexCapabilities(t *testing.T) {
	caps := New().Capabilities()
	if caps.SupportsSystemTransform {
		t.Fatalf("Codex exec hook should not report system transform support")
	}
	if !caps.SupportsExecHook {
		t.Fatalf("Codex adapter should support exec hook")
	}
	if caps.SupportsStreaming {
		t.Fatalf("Codex adapter should not support streaming in v1")
	}
	if caps.MaxHookTimeout <= 0 || caps.MaxHookTimeout > time.Second {
		t.Fatalf("unexpected MaxHookTimeout: %s", caps.MaxHookTimeout)
	}
}
