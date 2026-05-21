package mcp

import "context"

type ToolResult struct {
	Content string
	IsError bool
}

type Client interface {
	CallTool(ctx context.Context, name string, args map[string]any) (ToolResult, error)
	ListTools(ctx context.Context) ([]string, error)
	Close() error
}

type CallRequest struct {
	SessionID  string
	ServerName string
	ToolName   string
	Args       map[string]any
}

type Router struct {
	manager *Manager
	clients map[string]Client
}

func NewRouter(manager *Manager) *Router {
	return &Router{
		manager: manager,
		clients: map[string]Client{},
	}
}

func (r *Router) Register(sessionID, serverName string, client Client) {
	r.clients[clientKey(sessionID, serverName)] = client
}

func (r *Router) Call(ctx context.Context, req CallRequest) (ToolResult, error) {
	key := clientKey(req.SessionID, req.ServerName)
	c, ok := r.clients[key]
	if !ok {
		return ToolResult{}, &ErrServerNotFound{SessionID: req.SessionID, ServerName: req.ServerName}
	}
	return c.CallTool(ctx, req.ToolName, req.Args)
}

func (r *Router) ListTools(ctx context.Context, sessionID, serverName string) ([]string, error) {
	key := clientKey(sessionID, serverName)
	c, ok := r.clients[key]
	if !ok {
		return nil, &ErrServerNotFound{SessionID: sessionID, ServerName: serverName}
	}
	return c.ListTools(ctx)
}

func (r *Router) Disconnect(sessionID string) {
	for key, c := range r.clients {
		if len(key) > len(sessionID) && key[:len(sessionID)] == sessionID {
			_ = c.Close()
			delete(r.clients, key)
		}
	}
}

func (r *Router) ConnectedServers(sessionID string) []string {
	var out []string
	prefix := sessionID + ":"
	for key := range r.clients {
		if len(key) > len(prefix) && key[:len(prefix)] == prefix {
			out = append(out, key[len(prefix):])
		}
	}
	return out
}
