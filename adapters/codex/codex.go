package codex

import (
	"time"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/protocol"
)

func New() adapter.HostAdapter {
	return adapter.NewExecAdapter("codex", protocol.ProviderCodex, 800*time.Millisecond)
}
