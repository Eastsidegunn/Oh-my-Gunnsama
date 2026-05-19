package adapter

import (
	"context"
	"time"
)

// Worker is an OmG-managed agent runtime (e.g., a pi subprocess).
type Worker interface {
	Spawn(ctx context.Context, spec WorkerSpec) error
	Events() <-chan WorkerEvent
	Wait(ctx context.Context) (WorkerResult, error)
	Abort(ctx context.Context) error
}

type WorkerSpec struct {
	Goal       string            `json:"goal"`
	ProjectDir string            `json:"project_dir"`
	AllowTools []string          `json:"allow_tools"`
	Env        map[string]string `json:"env,omitempty"`
}

type WorkerEvent struct {
	Kind      string         `json:"kind"`
	Timestamp time.Time      `json:"timestamp"`
	Message   string         `json:"message,omitempty"`
	ToolName  string         `json:"tool_name,omitempty"`
	ToolArgs  map[string]any `json:"tool_args,omitempty"`
	Raw       []byte         `json:"-"`
	Sequence  int            `json:"sequence"`
}

type WorkerResult struct {
	ExitCode int    `json:"exit_code"`
	Success  bool   `json:"success"`
	Reason   string `json:"reason,omitempty"`
}
