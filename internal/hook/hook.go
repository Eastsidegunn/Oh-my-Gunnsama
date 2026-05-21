package hook

import (
	"context"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/protocol"
)

type Slot string

const (
	SlotInput     Slot = "input"
	SlotOutput    Slot = "output"
	SlotGuard     Slot = "guard"
	SlotLifecycle Slot = "lifecycle"
	SlotFlow      Slot = "flow"
)

// ToolCall carries tool execution context for SlotGuard hooks.
type ToolCall struct {
	Name   string
	Args   map[string]any
	Result any
	Phase  string // "before" | "after"
}

// SessionEvent carries session lifecycle context for SlotLifecycle hooks.
type SessionEvent struct {
	Kind      string // "error" | "stop" | "compact" | "idle"
	SessionID string
	Reason    string
}

// FlowDecision is the output of SlotFlow hooks.
type FlowDecision struct {
	Continue bool
	Reason   string
}

type Input struct {
	Slot         Slot
	AgentName    string
	Goal         string
	Spec         *adapter.WorkerSpec
	Event        *adapter.WorkerEvent
	Request      *protocol.Request
	ToolCall     *ToolCall
	SessionEvent *SessionEvent
}

type Output struct {
	Warnings     []string
	Allow        bool
	Reason       string
	FlowDecision *FlowDecision
}

type Hook interface {
	ID() string
	Slot() Slot
	Priority() int
	Apply(ctx context.Context, in *Input) (Output, error)
}

type HookFunc struct {
	IDValue       string
	SlotValue     Slot
	PriorityValue int
	Fn            func(context.Context, *Input) (Output, error)
}

func (h HookFunc) ID() string    { return h.IDValue }
func (h HookFunc) Slot() Slot    { return h.SlotValue }
func (h HookFunc) Priority() int { return h.PriorityValue }
func (h HookFunc) Apply(ctx context.Context, in *Input) (Output, error) {
	return h.Fn(ctx, in)
}

func NewFunc(id string, slot Slot, priority int, fn func(context.Context, *Input) (Output, error)) Hook {
	return HookFunc{IDValue: id, SlotValue: slot, PriorityValue: priority, Fn: fn}
}
