package mcp

import (
	"context"
	"errors"
	"testing"
)

type mockClient struct {
	tools   []string
	results map[string]ToolResult
	closed  bool
}

func (m *mockClient) CallTool(_ context.Context, name string, _ map[string]any) (ToolResult, error) {
	if r, ok := m.results[name]; ok {
		return r, nil
	}
	return ToolResult{}, errors.New("tool not found: " + name)
}

func (m *mockClient) ListTools(_ context.Context) ([]string, error) {
	return m.tools, nil
}

func (m *mockClient) Close() error {
	m.closed = true
	return nil
}

func TestRouter_CallRoutesToCorrectClient(t *testing.T) {
	router := NewRouter(nil)
	c := &mockClient{results: map[string]ToolResult{"search": {Content: "results"}}}
	router.Register("sess-1", "websearch", c)

	result, err := router.Call(context.Background(), CallRequest{
		SessionID:  "sess-1",
		ServerName: "websearch",
		ToolName:   "search",
		Args:       map[string]any{"q": "test"},
	})
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if result.Content != "results" {
		t.Errorf("Content=%q, want 'results'", result.Content)
	}
}

func TestRouter_CallReturnsErrServerNotFound(t *testing.T) {
	router := NewRouter(nil)
	_, err := router.Call(context.Background(), CallRequest{
		SessionID:  "sess-x",
		ServerName: "missing",
		ToolName:   "tool",
	})
	var notFound *ErrServerNotFound
	if !errors.As(err, &notFound) {
		t.Errorf("expected ErrServerNotFound, got %T: %v", err, err)
	}
}

func TestRouter_SessionIsolation(t *testing.T) {
	router := NewRouter(nil)
	c1 := &mockClient{results: map[string]ToolResult{"t": {Content: "sess1"}}}
	c2 := &mockClient{results: map[string]ToolResult{"t": {Content: "sess2"}}}
	router.Register("sess-1", "srv", c1)
	router.Register("sess-2", "srv", c2)

	r1, _ := router.Call(context.Background(), CallRequest{SessionID: "sess-1", ServerName: "srv", ToolName: "t"})
	r2, _ := router.Call(context.Background(), CallRequest{SessionID: "sess-2", ServerName: "srv", ToolName: "t"})
	if r1.Content != "sess1" || r2.Content != "sess2" {
		t.Errorf("isolation broken: r1=%q r2=%q", r1.Content, r2.Content)
	}
}

func TestRouter_DisconnectClosesSessionClients(t *testing.T) {
	router := NewRouter(nil)
	c1 := &mockClient{}
	c2 := &mockClient{}
	router.Register("sess-1", "srv-a", c1)
	router.Register("sess-1", "srv-b", c2)
	router.Register("sess-2", "srv-a", &mockClient{})

	router.Disconnect("sess-1")

	if !c1.closed || !c2.closed {
		t.Error("sess-1 clients should be closed")
	}
	servers := router.ConnectedServers("sess-2")
	if len(servers) != 1 || servers[0] != "srv-a" {
		t.Errorf("sess-2 should still have srv-a, got %v", servers)
	}
}

func TestRouter_ListTools(t *testing.T) {
	router := NewRouter(nil)
	c := &mockClient{tools: []string{"search", "fetch"}}
	router.Register("sess-1", "web", c)

	tools, err := router.ListTools(context.Background(), "sess-1", "web")
	if err != nil {
		t.Fatalf("ListTools error: %v", err)
	}
	if len(tools) != 2 || tools[0] != "search" {
		t.Errorf("tools=%v, want [search fetch]", tools)
	}
}

func TestRouter_ConnectedServers(t *testing.T) {
	router := NewRouter(nil)
	router.Register("sess-1", "websearch", &mockClient{})
	router.Register("sess-1", "context7", &mockClient{})
	router.Register("sess-2", "websearch", &mockClient{})

	servers := router.ConnectedServers("sess-1")
	if len(servers) != 2 {
		t.Errorf("expected 2 servers for sess-1, got %v", servers)
	}
}
