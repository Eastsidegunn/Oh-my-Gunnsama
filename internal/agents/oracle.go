package agents

import (
	"context"

	"oh-my-gunnsama/internal/hook"
	"oh-my-gunnsama/internal/registry"
)

func init() {
	if err := registry.Register(oracleAgent()); err != nil {
		panic(err)
	}
	registerHooks("oracle", []hook.Hook{oracleDenyWriteHook()})
}

func oracleAgent() registry.Agent {
	return registry.Agent{
		Name:        "oracle",
		Description: "Read-only consultant: high-IQ reasoning for debugging and architecture.",
		Category:    "advisor",
		Autonomy:    "cautious",
		Cost:        "expensive",
		Mode:        "subagent",
		Models: []registry.ModelCandidate{
			{Model: "openai/gpt-5.5", Variant: "high"},
		},
		Tools: registry.ToolPolicy{
			Mode:  registry.ToolModeAllowlist,
			Allow: []string{"read", "grep", "glob", "bash"},
		},
		Source: "code:internal/agents/oracle.go",
	}
}

func oracleDenyWriteHook() hook.Hook {
	denied := map[string]bool{"write": true, "edit": true, "task": true}
	return hook.NewFunc("oracle.deny-write", hook.SlotInput, 10,
		func(ctx context.Context, in *hook.Input) (hook.Output, error) {
			if in.Spec == nil {
				return hook.Output{Allow: true}, nil
			}
			for _, t := range in.Spec.AllowTools {
				if denied[t] {
					return hook.Output{
						Allow:  false,
						Reason: "oracle is read-only; tool denied: " + t,
					}, nil
				}
			}
			return hook.Output{Allow: true}, nil
		})
}
