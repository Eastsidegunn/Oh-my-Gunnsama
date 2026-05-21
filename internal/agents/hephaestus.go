package agents

import "oh-my-gunnsama/internal/registry"

func init() {
	if err := registry.Register(hephaestusAgent()); err != nil {
		panic(err)
	}
}

func hephaestusAgent() registry.Agent {
	return registry.Agent{
		Name:        "hephaestus",
		Description: "Autonomous executor: takes a goal and works it to completion.",
		Category:    "executor",
		Autonomy:    "high",
		Cost:        "medium",
		Mode:        "all",
		Models: []registry.ModelCandidate{
			{Model: "openai/gpt-5.5", Variant: "medium"},
		},
		Tools:  registry.ToolPolicy{Mode: registry.ToolModeAllExcept},
		Source: "code:internal/agents/hephaestus.go",
	}
}
