package agents

import (
	"context"
	"strings"
	"testing"

	"oh-my-gunnsama/internal/adapter"
	"oh-my-gunnsama/internal/config"
	"oh-my-gunnsama/internal/hook"
	"oh-my-gunnsama/internal/registry"
)

func buildFreshRegistry(t *testing.T) registry.Registry {
	t.Helper()
	reg, err := registry.Build(config.Config{}, registry.Options{})
	if err != nil {
		t.Fatalf("registry.Build error: %v", err)
	}
	return reg
}

func TestSisyphusHookDefinedSeparately(t *testing.T) {
	hooks := Hooks("sisyphus")
	if len(hooks) != 1 {
		t.Fatalf("hooks len=%d, want 1", len(hooks))
	}
	if hooks[0].ID() != "sisyphus.orch-rules" {
		t.Errorf("hook ID=%q, want sisyphus.orch-rules", hooks[0].ID())
	}
}

func TestHephaestusNoHooks(t *testing.T) {
	hooks := Hooks("hephaestus")
	if len(hooks) != 0 {
		t.Errorf("hooks len=%d, want 0", len(hooks))
	}
}

func TestOracleHookDefinedSeparately(t *testing.T) {
	hooks := Hooks("oracle")
	if len(hooks) != 1 {
		t.Fatalf("hooks len=%d, want 1", len(hooks))
	}
	if hooks[0].ID() != "oracle.deny-write" {
		t.Errorf("hook ID=%q, want oracle.deny-write", hooks[0].ID())
	}
}

func TestNonInvasiveGap_CoreRegistryDoesNotKnowAgents(t *testing.T) {
	reg := buildFreshRegistry(t)
	for _, name := range []string{"sisyphus", "hephaestus", "oracle"} {
		if _, ok := reg.Agent(name); ok {
			t.Errorf("verification gap: agent %q now visible via registry.Build() - core was modified", name)
		}
	}
}

func TestNonInvasiveGap_AllAgentsKnownLocally(t *testing.T) {
	all := AllAgents()
	names := map[string]bool{}
	for _, n := range all {
		names[n] = true
	}
	for _, expected := range []string{"sisyphus", "oracle"} {
		if !names[expected] {
			t.Errorf("agent %q should be in local map", expected)
		}
	}
}

func TestSisyphusHook_PrefixesGoalAndSetsEnv(t *testing.T) {
	h := sisyphusOrchRulesHook()
	spec := &adapter.WorkerSpec{Goal: "do thing"}
	in := &hook.Input{Slot: hook.SlotInput, Spec: spec}
	out, err := h.Apply(context.Background(), in)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !out.Allow {
		t.Errorf("Allow=false, want true")
	}
	if !strings.HasPrefix(spec.Goal, "[SISYPHUS] ") {
		t.Errorf("goal=%q, want [SISYPHUS] prefix", spec.Goal)
	}
	if spec.Env["OMG_AGENT_ROLE"] != "orchestrator" {
		t.Errorf("OMG_AGENT_ROLE=%q, want orchestrator", spec.Env["OMG_AGENT_ROLE"])
	}
}

func TestOracleHook_BlocksWrite(t *testing.T) {
	h := oracleDenyWriteHook()
	spec := &adapter.WorkerSpec{AllowTools: []string{"read", "write"}}
	in := &hook.Input{Slot: hook.SlotInput, Spec: spec}
	out, err := h.Apply(context.Background(), in)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if out.Allow {
		t.Errorf("Allow=true, want false (write should be denied)")
	}
	if !strings.Contains(out.Reason, "write") {
		t.Errorf("Reason=%q, want mention of write", out.Reason)
	}
}

func TestOracleHook_BlocksEdit(t *testing.T) {
	h := oracleDenyWriteHook()
	spec := &adapter.WorkerSpec{AllowTools: []string{"edit"}}
	in := &hook.Input{Slot: hook.SlotInput, Spec: spec}
	out, _ := h.Apply(context.Background(), in)
	if out.Allow {
		t.Errorf("edit should be denied")
	}
}

func TestOracleHook_AllowsReadOnly(t *testing.T) {
	h := oracleDenyWriteHook()
	spec := &adapter.WorkerSpec{AllowTools: []string{"read", "grep", "glob"}}
	in := &hook.Input{Slot: hook.SlotInput, Spec: spec}
	out, err := h.Apply(context.Background(), in)
	if err != nil {
		t.Fatalf("Apply error: %v", err)
	}
	if !out.Allow {
		t.Errorf("read-only tools should be allowed, Reason=%q", out.Reason)
	}
}

func TestAllAgents_ToolPolicyMatchesSpec(t *testing.T) {
	cases := []struct {
		agent    registry.Agent
		wantMode registry.ToolMode
	}{
		{sisyphusAgent(), registry.ToolModeAllExcept},
		{hephaestusAgent(), registry.ToolModeAllExcept},
		{oracleAgent(), registry.ToolModeAllowlist},
	}
	for _, c := range cases {
		if c.agent.Tools.Mode != c.wantMode {
			t.Errorf("agent %q tool mode=%q, want %q", c.agent.Name, c.agent.Tools.Mode, c.wantMode)
		}
	}
}
