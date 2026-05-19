// Package protocol defines Oh-My-Gunnsama's host-neutral v1 wire envelope.
//
// The Module's Interface is intentionally small: decode one JSON request,
// validate the stable top-level fields, and encode one JSON response. Host
// specific data belongs in Payload or Capabilities and is opaque here.
package protocol

import (
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
)

const Version = 1

type Provider string

const (
	ProviderCodex    Provider = "codex"
	ProviderClaude   Provider = "claude"
	ProviderOpenCode Provider = "opencode"
	ProviderDirect   Provider = "direct"
)

type Event string

const (
	EventOnSubmit     Event = "on-submit"
	EventOnToolBefore Event = "on-tool-before"
	EventOnToolAfter  Event = "on-tool-after"
	EventOnError      Event = "on-error"
	EventOnCompact    Event = "on-compact"
	EventOnStop       Event = "on-stop"
	EventOnRun        Event = "on-run"
)

type Action string

const (
	ActionNone         Action = "none"
	ActionInjectPrompt Action = "inject_prompt"
	ActionBlock        Action = "block"
	ActionReplace      Action = "replace"
	ActionWarn         Action = "warn"
)

type ErrorClass string

const (
	ErrorTimeout           ErrorClass = "timeout"
	ErrorInvalidConfig     ErrorClass = "invalid_config"
	ErrorSocketUnavailable ErrorClass = "socket_unavailable"
	ErrorPolicyBlocked     ErrorClass = "policy_blocked"
	ErrorFilesystem        ErrorClass = "filesystem_error"
	ErrorUnknown           ErrorClass = "unknown"
)

type Request struct {
	Version      int            `json:"version"`
	RequestID    string         `json:"request_id"`
	Provider     Provider       `json:"provider"`
	Event        Event          `json:"event"`
	Project      string         `json:"project"`
	CWD          string         `json:"cwd"`
	SessionID    string         `json:"session_id,omitempty"`
	TimeoutMS    int            `json:"timeout_ms,omitempty"`
	Capabilities map[string]any `json:"capabilities,omitempty"`
	Payload      map[string]any `json:"payload,omitempty"`
}

type Response struct {
	OK        bool           `json:"ok"`
	RequestID string         `json:"request_id,omitempty"`
	Action    Action         `json:"action"`
	Output    string         `json:"output"`
	Warnings  []string       `json:"warnings"`
	Error     *ResponseError `json:"error"`
}

type ResponseError struct {
	Class     ErrorClass `json:"class"`
	Message   string     `json:"message"`
	Retryable bool       `json:"retryable"`
}

type ProtocolError struct {
	Class     ErrorClass
	Message   string
	Retryable bool
}

func (e *ProtocolError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

func ReadRequest(r io.Reader) (*Request, *ProtocolError) {
	var req Request
	decoder := json.NewDecoder(r)
	if err := decoder.Decode(&req); err != nil {
		return nil, NewProtocolError(ErrorInvalidConfig, fmt.Sprintf("decode request: %v", err), false)
	}
	if err := ValidateRequest(&req); err != nil {
		return &req, err
	}
	return &req, nil
}

func WriteResponse(w io.Writer, resp Response) error {
	if resp.Warnings == nil {
		resp.Warnings = []string{}
	}
	encoder := json.NewEncoder(w)
	return encoder.Encode(resp)
}

func OKResponse(req *Request, action Action, output string, warnings []string) Response {
	return Response{
		OK:        true,
		RequestID: requestIDOf(req),
		Action:    action,
		Output:    output,
		Warnings:  normalizeWarnings(warnings),
		Error:     nil,
	}
}

func ErrorResponse(req *Request, action Action, err *ProtocolError, warnings []string) Response {
	if action == "" {
		action = ActionWarn
	}
	if err == nil {
		err = NewProtocolError(ErrorUnknown, "unknown protocol error", false)
	}
	return Response{
		OK:        false,
		RequestID: requestIDOf(req),
		Action:    action,
		Output:    "",
		Warnings:  normalizeWarnings(warnings),
		Error: &ResponseError{
			Class:     err.Class,
			Message:   err.Message,
			Retryable: err.Retryable,
		},
	}
}

func NewProtocolError(class ErrorClass, message string, retryable bool) *ProtocolError {
	if class == "" {
		class = ErrorUnknown
	}
	if message == "" {
		message = "unknown protocol error"
	}
	return &ProtocolError{Class: class, Message: message, Retryable: retryable}
}

func ValidateRequest(req *Request) *ProtocolError {
	if req == nil {
		return NewProtocolError(ErrorInvalidConfig, "request is required", false)
	}
	if req.Version != Version {
		return NewProtocolError(ErrorInvalidConfig, "version must be 1", false)
	}
	if req.RequestID == "" {
		return NewProtocolError(ErrorInvalidConfig, "request_id is required", false)
	}
	if !validProvider(req.Provider) {
		return NewProtocolError(ErrorInvalidConfig, "provider must be codex, claude, opencode, or direct", false)
	}
	if !validEvent(req.Event) {
		return NewProtocolError(ErrorInvalidConfig, "event is not supported by protocol v1", false)
	}
	if req.Project != "" && !filepath.IsAbs(req.Project) {
		return NewProtocolError(ErrorInvalidConfig, "project must be an absolute path when provided", false)
	}
	if req.CWD != "" && !filepath.IsAbs(req.CWD) {
		return NewProtocolError(ErrorInvalidConfig, "cwd must be an absolute path when provided", false)
	}
	return nil
}

func requestIDOf(req *Request) string {
	if req == nil {
		return ""
	}
	return req.RequestID
}

func normalizeWarnings(warnings []string) []string {
	if warnings == nil {
		return []string{}
	}
	return warnings
}

func validProvider(provider Provider) bool {
	switch provider {
	case ProviderCodex, ProviderClaude, ProviderOpenCode, ProviderDirect:
		return true
	default:
		return false
	}
}

func validEvent(event Event) bool {
	switch event {
	case EventOnSubmit, EventOnToolBefore, EventOnToolAfter, EventOnError, EventOnCompact, EventOnStop, EventOnRun:
		return true
	default:
		return false
	}
}
