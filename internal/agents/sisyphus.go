package agents

import (
	"context"

	"oh-my-gunnsama/internal/hook"
	"oh-my-gunnsama/internal/registry"
)

func init() {
	if err := registry.Register(sisyphusAgent()); err != nil {
		panic(err)
	}
	registerHooks("sisyphus", []hook.Hook{sisyphusOrchRulesHook()})
}

func sisyphusAgent() registry.Agent {
	return registry.Agent{
		Name:        "sisyphus",
		Description: "Orchestrator: dispatches sub-work to executors and verifies outcomes.",
		Category:    "orchestrator",
		Autonomy:    "cautious",
		Cost:        "expensive",
		Mode:        "primary",
		Models: []registry.ModelCandidate{
			{Model: "anthropic/claude-opus-4-7", Variant: "high"},
		},
		Tools:  registry.ToolPolicy{Mode: registry.ToolModeAllExcept},
		Source: "code:internal/agents/sisyphus.go",
	}
}

func sisyphusOrchRulesHook() hook.Hook {
	return hook.NewFunc("sisyphus.orch-rules", hook.SlotInput, 10,
		func(ctx context.Context, in *hook.Input) (hook.Output, error) {
			if in.Spec == nil {
				return hook.Output{Allow: true}, nil
			}
			if in.Spec.Env == nil {
				in.Spec.Env = map[string]string{}
			}
			in.Spec.Env["OMG_AGENT_ROLE"] = "orchestrator"
			in.Spec.Goal = "[SISYPHUS] " + in.Spec.Goal
			return hook.Output{
				Allow:    true,
				Warnings: []string{"sisyphus orchestrator rules applied"},
			}, nil
		})
}
