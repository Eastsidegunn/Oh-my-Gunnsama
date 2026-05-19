package adapter

import (
	"time"

	"oh-my-gunnsama/internal/protocol"
)

type HostAdapter interface {
	Name() string
	Capabilities() HostCapabilities
	TranslateEvent(raw []byte) (*protocol.Request, error)
	TranslateResponse(resp protocol.Response) ([]byte, error)
	HealthCheck() HostStatus
}

type HostCapabilities struct {
	SupportsSystemTransform bool
	SupportsExecHook        bool
	MaxHookTimeout          time.Duration
	SupportsStreaming       bool
}

type HostStatus struct {
	Name      string
	Connected bool
	Degraded  bool
	Message   string
}
