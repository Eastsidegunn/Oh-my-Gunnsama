package route

import (
	"testing"

	"oh-my-gunnsama/internal/registry"
)

func TestRouteMatchesRegistryTriggerAndSelectsAgent(t *testing.T) {
	reg := registry.Registry{Agents: map[string]registry.Agent{
		"architect": {
			Name:     "architect",
			Category: "advisor",
			Autonomy: "cautious",
			Triggers: []registry.Trigger{{
				Domain:   "architecture",
				Pattern:  "architecture|seam|interface",
				Priority: 10,
			}},
		},
	}}

	result := Route("review the architecture seam for the daemon interface", reg)
	if result.Procedure != ProcedureReview {
		t.Fatalf("Procedure = %q, want review", result.Procedure)
	}
	if result.Agent != "architect" || result.Domain != "architecture" {
		t.Fatalf("agent/domain mismatch: %#v", result)
	}
	if result.Confidence != ConfidenceHigh {
		t.Fatalf("Confidence = %q, want high", result.Confidence)
	}
	if result.Score == nil || *result.Score <= 0 {
		t.Fatalf("Score not populated: %#v", result.Score)
	}
	if len(result.Triggers) == 0 {
		t.Fatalf("matched triggers not recorded: %#v", result)
	}
}

func TestRouteConfidenceLevels(t *testing.T) {
	reg := registry.Registry{Agents: map[string]registry.Agent{
		"planner": {
			Name:     "planner",
			Category: "planning",
			Autonomy: "cautious",
			Triggers: []registry.Trigger{{
				Domain:   "planning",
				Pattern:  "plan|roadmap",
				Priority: 10,
			}},
		},
	}}

	medium := Route("make a plan for the cli", reg)
	if medium.Confidence != ConfidenceMedium {
		t.Fatalf("single trigger Confidence = %q, want medium (%#v)", medium.Confidence, medium)
	}

	high := Route("make a plan and roadmap for the cli", reg)
	if high.Confidence != ConfidenceHigh {
		t.Fatalf("two trigger Confidence = %q, want high (%#v)", high.Confidence, high)
	}
}

func TestRouteRequiresUserForCautiousAgentLowConfidence(t *testing.T) {
	reg := registry.Registry{Agents: map[string]registry.Agent{
		"architect": {
			Name:     "architect",
			Category: "advisor",
			Autonomy: "cautious",
			Triggers: []registry.Trigger{{
				Domain:   "architecture",
				Pattern:  "architecture",
				Priority: 10,
			}},
		},
	}}

	result := Route("maybe architecture?", reg)
	if result.Confidence != ConfidenceLow {
		t.Fatalf("Confidence = %q, want low (%#v)", result.Confidence, result)
	}
	if !result.RequiresUser {
		t.Fatalf("RequiresUser = false, want true for cautious low-confidence route: %#v", result)
	}
}

func TestRouteNoMatchReturnsProcedureNone(t *testing.T) {
	reg := registry.Registry{Agents: map[string]registry.Agent{
		"executor": {
			Name:     "executor",
			Category: "execution",
			Autonomy: "autonomous",
			Triggers: []registry.Trigger{{Domain: "execution", Pattern: "implement", Priority: 1}},
		},
	}}

	result := Route("hello there", reg)
	if result.Procedure != ProcedureNone {
		t.Fatalf("Procedure = %q, want none (%#v)", result.Procedure, result)
	}
	if result.Agent != "" {
		t.Fatalf("Agent = %q, want empty", result.Agent)
	}
	if result.RequiresUser {
		t.Fatalf("RequiresUser = true, want false for no match")
	}
}
