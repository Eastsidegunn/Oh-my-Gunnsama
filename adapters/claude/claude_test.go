package claude

import (
	"encoding/json"
	"testing"
	"time"

	"oh-my-gunnsama/internal/protocol"
)

func TestTranslateEventConvertsClaudePayloadToEnvelope(t *testing.T) {
	adapter := New()
	raw := []byte(`{
		"request_id":"claude-1",
		"event":"UserPromptSubmit",
		"prompt":"hello",
		"project":"/repo",
		"cwd":"/repo",
		"session_id":"session-1",
		"timeout_ms":400
	}`)

	req, err := adapter.TranslateEvent(raw)
	if err != nil {
		t.Fatalf("TranslateEvent returned error: %v", err)
	}
	if req.Version != protocol.Version || req.Provider != protocol.ProviderClaude || req.Event != protocol.EventOnSubmit {
		t.Fatalf("bad envelope: %#v", req)
	}
	if req.RequestID != "claude-1" || req.Project != "/repo" || req.CWD != "/repo" || req.SessionID != "session-1" || req.TimeoutMS != 400 {
		t.Fatalf("bad stable fields: %#v", req)
	}
	if req.Payload["prompt"] != "hello" {
		t.Fatalf("payload prompt not preserved: %#v", req.Payload)
	}
}

func TestTranslateResponseAppliesInjectPromptToClaudeHostFormat(t *testing.T) {
	adapter := New()
	out, err := adapter.TranslateResponse(protocol.Response{OK: true, RequestID: "r", Action: protocol.ActionInjectPrompt, Output: "SYSTEM", Warnings: []string{"w"}})
	if err != nil {
		t.Fatalf("TranslateResponse returned error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("output is not JSON: %v", err)
	}
	if decoded["action"] != "inject_prompt" || decoded["system_prompt"] != "SYSTEM" {
		t.Fatalf("bad host output: %#v", decoded)
	}
}

func TestClaudeCapabilities(t *testing.T) {
	caps := New().Capabilities()
	if caps.SupportsSystemTransform {
		t.Fatalf("Claude exec hook should not report system transform support")
	}
	if !caps.SupportsExecHook {
		t.Fatalf("Claude adapter should support exec hook")
	}
	if caps.SupportsStreaming {
		t.Fatalf("Claude adapter should not support streaming in v1")
	}
	if caps.MaxHookTimeout <= 0 || caps.MaxHookTimeout > time.Second {
		t.Fatalf("unexpected MaxHookTimeout: %s", caps.MaxHookTimeout)
	}
}
