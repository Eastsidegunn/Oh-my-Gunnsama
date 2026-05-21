package mcp

import "fmt"

type ErrServerNotFound struct {
	SessionID  string
	ServerName string
}

func (e *ErrServerNotFound) Error() string {
	return fmt.Sprintf("mcp server %q not connected for session %q", e.ServerName, e.SessionID)
}
