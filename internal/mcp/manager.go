package mcp

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type ServerConfig struct {
	Name       string
	Command    []string
	Tools      []string
	EnvInherit []string
}

type Process interface {
	PID() int
	Alive() bool
	Stop() error
}

type Starter interface {
	Start(config ServerConfig, env map[string]string) (Process, error)
}

type Options struct {
	Starter     Starter
	IdleTimeout time.Duration
	Env         map[string]string
}

type Manager struct {
	starter     Starter
	idleTimeout time.Duration
	env         map[string]string
	clients     map[string]*managedProcess
	mu          sync.Mutex
}

type managedProcess struct {
	process    Process
	lastUsedAt time.Time
}

func NewManager(options Options) *Manager {
	if options.IdleTimeout == 0 {
		options.IdleTimeout = 5 * time.Minute
	}
	return &Manager{
		starter:     options.Starter,
		idleTimeout: options.IdleTimeout,
		env:         cloneMap(options.Env),
		clients:     map[string]*managedProcess{},
	}
}

func (m *Manager) Use(sessionID string, config ServerConfig, now time.Time) (Process, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	key := clientKey(sessionID, config.Name)
	if managed := m.clients[key]; managed != nil && managed.process.Alive() {
		managed.lastUsedAt = now
		return managed.process, nil
	}
	if m.starter == nil {
		return nil, fmt.Errorf("mcp starter is required")
	}
	process, err := m.starter.Start(config, m.allowlistedEnv(config.EnvInherit))
	if err != nil {
		return nil, err
	}
	m.clients[key] = &managedProcess{process: process, lastUsedAt: now}
	return process, nil
}

func (m *Manager) CleanupIdle(now time.Time) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	removed := 0
	for key, managed := range m.clients {
		if now.Sub(managed.lastUsedAt) <= m.idleTimeout {
			continue
		}
		_ = managed.process.Stop()
		delete(m.clients, key)
		removed++
	}
	return removed
}

func (m *Manager) CleanupZombies() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	removed := 0
	for key, managed := range m.clients {
		if managed.process.Alive() {
			continue
		}
		delete(m.clients, key)
		removed++
	}
	return removed
}

func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.clients)
}

func (m *Manager) allowlistedEnv(names []string) map[string]string {
	out := map[string]string{}
	for _, name := range names {
		if value, ok := m.env[name]; ok {
			out[name] = value
		}
	}
	return out
}

func clientKey(sessionID, serverName string) string {
	return strings.Join([]string{sessionID, serverName}, ":")
}

func cloneMap(values map[string]string) map[string]string {
	if values == nil {
		return map[string]string{}
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
