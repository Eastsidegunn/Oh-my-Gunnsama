package mcp

import (
	"testing"
	"time"
)

func TestManagerStartsServerLazilyOnFirstUse(t *testing.T) {
	starter := &fakeStarter{}
	manager := NewManager(Options{Starter: starter, IdleTimeout: 5 * time.Minute, Env: map[string]string{"PATH": "/bin"}})
	config := ServerConfig{Name: "ast", EnvInherit: []string{"PATH"}}

	if starter.starts != 0 {
		t.Fatalf("server started before use")
	}
	client, err := manager.Use("session-1", config, time.Now())
	if err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	if starter.starts != 1 || client.PID() == 0 {
		t.Fatalf("lazy start failed: starts=%d pid=%d", starter.starts, client.PID())
	}
	_, err = manager.Use("session-1", config, time.Now().Add(time.Second))
	if err != nil {
		t.Fatalf("second Use returned error: %v", err)
	}
	if starter.starts != 1 {
		t.Fatalf("expected process reuse, starts=%d", starter.starts)
	}
}

func TestManagerShutsDownIdleServerAfterTimeout(t *testing.T) {
	starter := &fakeStarter{}
	manager := NewManager(Options{Starter: starter, IdleTimeout: time.Minute})
	config := ServerConfig{Name: "ast"}
	now := time.Now()
	proc, err := manager.Use("session-1", config, now)
	if err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	manager.CleanupIdle(now.Add(time.Minute + time.Second))
	if !proc.(*fakeProcess).stopped {
		t.Fatalf("idle process was not stopped")
	}
	if manager.ActiveCount() != 0 {
		t.Fatalf("active count = %d, want 0", manager.ActiveCount())
	}
}

func TestManagerPassesOnlyAllowlistedEnvironment(t *testing.T) {
	starter := &fakeStarter{}
	manager := NewManager(Options{Starter: starter, Env: map[string]string{"PATH": "/bin", "SECRET": "nope"}})
	_, err := manager.Use("session-1", ServerConfig{Name: "ast", EnvInherit: []string{"PATH"}}, time.Now())
	if err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	if starter.lastEnv["PATH"] != "/bin" {
		t.Fatalf("PATH not inherited: %#v", starter.lastEnv)
	}
	if _, ok := starter.lastEnv["SECRET"]; ok {
		t.Fatalf("non-allowlisted SECRET was inherited: %#v", starter.lastEnv)
	}
}

func TestManagerDetectsZombiePIDAndRestartsOnNextUse(t *testing.T) {
	starter := &fakeStarter{}
	manager := NewManager(Options{Starter: starter})
	config := ServerConfig{Name: "ast"}
	now := time.Now()
	proc, err := manager.Use("session-1", config, now)
	if err != nil {
		t.Fatalf("Use returned error: %v", err)
	}
	proc.(*fakeProcess).alive = false
	removed := manager.CleanupZombies()
	if removed != 1 {
		t.Fatalf("removed = %d, want 1", removed)
	}
	_, err = manager.Use("session-1", config, now.Add(time.Second))
	if err != nil {
		t.Fatalf("Use after zombie returned error: %v", err)
	}
	if starter.starts != 2 {
		t.Fatalf("expected restart after zombie cleanup, starts=%d", starter.starts)
	}
}

type fakeStarter struct {
	starts  int
	lastEnv map[string]string
}

func (f *fakeStarter) Start(config ServerConfig, env map[string]string) (Process, error) {
	f.starts++
	f.lastEnv = env
	return &fakeProcess{pid: f.starts, alive: true}, nil
}

type fakeProcess struct {
	pid     int
	alive   bool
	stopped bool
}

func (f *fakeProcess) PID() int    { return f.pid }
func (f *fakeProcess) Alive() bool { return f.alive }
func (f *fakeProcess) Stop() error { f.stopped = true; f.alive = false; return nil }
