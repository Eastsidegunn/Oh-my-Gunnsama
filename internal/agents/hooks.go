package agents

import (
	"sync"

	"oh-my-gunnsama/internal/hook"
)

var (
	hookMu        sync.RWMutex
	agentHooksMap = map[string][]hook.Hook{}
)

func registerHooks(agentName string, hooks []hook.Hook) {
	hookMu.Lock()
	defer hookMu.Unlock()
	agentHooksMap[agentName] = hooks
}

func Hooks(agentName string) []hook.Hook {
	hookMu.RLock()
	defer hookMu.RUnlock()
	hooks, ok := agentHooksMap[agentName]
	if !ok {
		return nil
	}
	out := make([]hook.Hook, len(hooks))
	copy(out, hooks)
	return out
}

func AllAgents() []string {
	hookMu.RLock()
	defer hookMu.RUnlock()
	out := make([]string, 0, len(agentHooksMap))
	for name := range agentHooksMap {
		out = append(out, name)
	}
	return out
}
