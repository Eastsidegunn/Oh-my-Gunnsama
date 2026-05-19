package adapter

import (
	"encoding/json"
	"fmt"
	"time"

	"oh-my-gunnsama/internal/protocol"
)

type ExecAdapter struct {
	name       string
	provider   protocol.Provider
	maxTimeout time.Duration
}

type ExecHostEvent struct {
	RequestID string         `json:"request_id"`
	Event     string         `json:"event"`
	Prompt    string         `json:"prompt,omitempty"`
	Input     string         `json:"input,omitempty"`
	Project   string         `json:"project"`
	CWD       string         `json:"cwd"`
	SessionID string         `json:"session_id,omitempty"`
	TimeoutMS int            `json:"timeout_ms,omitempty"`
	Raw       map[string]any `json:"-"`
}

func NewExecAdapter(name string, provider protocol.Provider, maxTimeout time.Duration) ExecAdapter {
	return ExecAdapter{name: name, provider: provider, maxTimeout: maxTimeout}
}

func (a ExecAdapter) Name() string {
	return a.name
}

func (a ExecAdapter) Capabilities() HostCapabilities {
	return HostCapabilities{
		SupportsSystemTransform: false,
		SupportsExecHook:        true,
		MaxHookTimeout:          a.maxTimeout,
		SupportsStreaming:       false,
	}
}

func (a ExecAdapter) TranslateEvent(raw []byte) (*protocol.Request, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode %s event: %w", a.name, err)
	}
	var event ExecHostEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		return nil, fmt.Errorf("decode %s event fields: %w", a.name, err)
	}
	request := &protocol.Request{
		Version:      protocol.Version,
		RequestID:    event.RequestID,
		Provider:     a.provider,
		Event:        normalizeEvent(event.Event),
		Project:      event.Project,
		CWD:          event.CWD,
		SessionID:    event.SessionID,
		TimeoutMS:    event.TimeoutMS,
		Capabilities: map[string]any{"supports_exec_hook": true},
		Payload:      payload,
	}
	if err := protocol.ValidateRequest(request); err != nil {
		return request, err
	}
	return request, nil
}

func (a ExecAdapter) TranslateResponse(resp protocol.Response) ([]byte, error) {
	message := resp.Output
	if resp.Error != nil && message == "" {
		message = resp.Error.Message
	}
	host := map[string]any{
		"ok":         resp.OK,
		"request_id": resp.RequestID,
		"action":     string(resp.Action),
		"message":    message,
		"warnings":   resp.Warnings,
	}
	if resp.Action == protocol.ActionInjectPrompt {
		host["system_prompt"] = resp.Output
	}
	if resp.Error != nil {
		host["error"] = resp.Error
	}
	return json.Marshal(host)
}

func (a ExecAdapter) HealthCheck() HostStatus {
	return HostStatus{Name: a.name, Connected: true}
}

func normalizeEvent(event string) protocol.Event {
	switch event {
	case "UserPromptSubmit", "on-submit", "":
		return protocol.EventOnSubmit
	case "PreToolUse", "tool.execute.before", "on-tool-before":
		return protocol.EventOnToolBefore
	case "PostToolUse", "tool.execute.after", "on-tool-after":
		return protocol.EventOnToolAfter
	case "on-error":
		return protocol.EventOnError
	case "on-compact":
		return protocol.EventOnCompact
	case "on-stop":
		return protocol.EventOnStop
	default:
		return protocol.Event(event)
	}
}
