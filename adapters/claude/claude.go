package claude

import (
	"time"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/protocol"
)

func New() adapter.HostAdapter {
	return adapter.NewExecAdapter("claude", protocol.ProviderClaude, 800*time.Millisecond)
}
