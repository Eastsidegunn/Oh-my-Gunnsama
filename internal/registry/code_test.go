package registry

import (
	"strings"
	"testing"

	"oh-my-gunnsama/internal/config"
)

func TestRegister_AddsAgentToCodeRegistry(t *testing.T) {
	t.Cleanup(ResetRegistry)
	if err := Register(Agent{Name: "alpha", Description: "Alpha agent"}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	got := registered()
	if _, ok := got["alpha"]; !ok {
		t.Errorf("agent alpha not in registered map")
	}
}

func TestRegister_RejectsEmptyName(t *testing.T) {
	t.Cleanup(ResetRegistry)
	if err := Register(Agent{Description: "no name"}); err == nil {
		t.Errorf("expected error for empty name")
	}
}

func TestRegister_RejectsDuplicateName(t *testing.T) {
	t.Cleanup(ResetRegistry)
	if err := Register(Agent{Name: "dup", Description: "first"}); err != nil {
		t.Fatalf("first Register error: %v", err)
	}
	err := Register(Agent{Name: "dup", Description: "second"})
	if err == nil || !strings.Contains(err.Error(), "already registered") {
		t.Errorf("expected duplicate error, got %v", err)
	}
}

func TestRegister_StoresInCodeAgentsMap(t *testing.T) {
	t.Cleanup(ResetRegistry)
	if err := Register(Agent{Name: "sisyphus", Description: "Orchestrator"}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	got := registered()
	if _, ok := got["sisyphus"]; !ok {
		t.Errorf("Register should populate codeAgents map")
	}
}

func TestNonInvasiveGap_BuildDoesNotMergeCodeAgents(t *testing.T) {
	t.Cleanup(ResetRegistry)
	if err := Register(Agent{Name: "code-only", Description: "Only in code"}); err != nil {
		t.Fatalf("Register error: %v", err)
	}
	reg, err := Build(config.Config{}, Options{})
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if _, ok := reg.Agent("code-only"); ok {
		t.Error("verification gap: Build() now sees code agents - core was modified")
	}
}

func TestBuild_NoCodeAgents_BehavesAsBefore(t *testing.T) {
	t.Cleanup(ResetRegistry)
	cfg := config.Config{Agents: map[string]config.AgentDefinition{
		"yaml-only": {Name: "yaml-only", Description: "YAML"},
	}}
	reg, err := Build(cfg, Options{})
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}
	if _, ok := reg.Agent("yaml-only"); !ok {
		t.Errorf("YAML-only agent should remain accessible")
	}
}
