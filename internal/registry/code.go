package registry

import (
	"errors"
	"fmt"
	"sync"
)

var (
	codeMu     sync.RWMutex
	codeAgents = map[string]Agent{}
)

// Register adds an agent to the code-defined registry. Called from package
// init() in agent definition files (e.g., internal/agents/sisyphus.go).
//
// Code-defined agents take precedence over YAML-defined agents with the
// same name, except when YAML sets disabled: true (which suppresses both).
func Register(a Agent) error {
	if a.Name == "" {
		return errors.New("agent name is required")
	}
	codeMu.Lock()
	defer codeMu.Unlock()
	if _, exists := codeAgents[a.Name]; exists {
		return fmt.Errorf("agent %q already registered", a.Name)
	}
	codeAgents[a.Name] = a
	return nil
}

func registered() map[string]Agent {
	codeMu.RLock()
	defer codeMu.RUnlock()
	out := make(map[string]Agent, len(codeAgents))
	for k, v := range codeAgents {
		out[k] = v
	}
	return out
}

// ResetRegistry clears all code-registered agents. Test-only helper for
// hermetic isolation between test cases.
func ResetRegistry() {
	codeMu.Lock()
	defer codeMu.Unlock()
	codeAgents = map[string]Agent{}
}
