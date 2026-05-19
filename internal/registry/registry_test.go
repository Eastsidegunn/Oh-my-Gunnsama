package registry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oh-my-gunnsama/internal/config"
)

func TestBuildLoadsActiveAgentsAndNormalizesMergedDefinitions(t *testing.T) {
	cfg := config.Config{Agents: map[string]config.AgentDefinition{
		"architect": {
			Name:        "architect",
			Description: "Project architect",
			Aliases:     []string{"oracle", "arch"},
			Category:    "advisor",
			Autonomy:    "cautious",
			Cost:        "expensive",
			Mode:        "subagent",
			Model:       "anthropic/claude-opus-4-7",
			Tools:       []string{"read", "grep", "glob"},
			Triggers: []config.TriggerDefinition{{
				Domain:   "Architecture decisions",
				Pattern:  "architecture|seam|interface",
				Priority: 10,
			}},
			UseWhen:   []string{"Complex Module design"},
			AvoidWhen: []string{"Simple file edits"},
		},
	}}

	reg, err := Build(cfg, Options{})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}

	agent, ok := reg.Agent("architect")
	if !ok {
		t.Fatalf("architect agent was not loaded")
	}
	if agent.Description != "Project architect" || agent.Category != "advisor" || agent.Autonomy != "cautious" {
		t.Fatalf("agent fields not normalized: %#v", agent)
	}
	if got := agent.Tools; got.Mode != ToolModeAllowlist || !equalStrings(got.Allow, []string{"read", "grep", "glob"}) {
		t.Fatalf("tool policy = %#v", got)
	}
	if got, ok := reg.ResolveAlias("oracle"); !ok || got != "architect" {
		t.Fatalf("alias oracle resolved to %q, %v", got, ok)
	}
}

func TestBuildRejectsAliasCollision(t *testing.T) {
	cfg := config.Config{Agents: map[string]config.AgentDefinition{
		"architect": {Name: "architect", Description: "Architect", Aliases: []string{"oracle"}},
		"critic":    {Name: "critic", Description: "Critic", Aliases: []string{"oracle"}},
	}}

	_, err := Build(cfg, Options{})
	if err == nil || !strings.Contains(err.Error(), "alias collision") {
		t.Fatalf("Build error = %v, want alias collision", err)
	}
}

func TestBuildExcludesDisabledAgent(t *testing.T) {
	cfg := config.Config{Agents: map[string]config.AgentDefinition{
		"experimental": {Name: "experimental", Description: "Experiment", Disabled: true, Aliases: []string{"exp"}},
	}}

	reg, err := Build(cfg, Options{})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if _, ok := reg.Agent("experimental"); ok {
		t.Fatalf("disabled agent should not be active")
	}
	if _, ok := reg.ResolveAlias("exp"); ok {
		t.Fatalf("disabled agent alias should not be active")
	}
}

func TestBuildWarnsOnUnknownFieldAndStrictModeFails(t *testing.T) {
	cfg := config.Config{Agents: map[string]config.AgentDefinition{
		"architect": {Name: "architect", Description: "Architect", UnknownFields: []string{"posture"}},
	}}

	reg, err := Build(cfg, Options{})
	if err != nil {
		t.Fatalf("Build returned error in non-strict mode: %v", err)
	}
	if len(reg.Warnings) != 1 || !strings.Contains(reg.Warnings[0], "posture") {
		t.Fatalf("warnings = %#v, want unknown field warning", reg.Warnings)
	}

	_, err = Build(cfg, Options{Strict: true})
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("strict Build error = %v, want unknown field fatal", err)
	}
}

func TestBuildRejectsToolsExceptWithAllowlist(t *testing.T) {
	cfg := config.Config{Agents: map[string]config.AgentDefinition{
		"bad-tools": {Name: "bad-tools", Description: "Bad", Tools: []string{"read"}, ToolsExcept: []string{"write"}},
	}}

	_, err := Build(cfg, Options{})
	if err == nil || !strings.Contains(err.Error(), "tools_except") {
		t.Fatalf("Build error = %v, want tools_except validation error", err)
	}
}

func TestConfigLoadFeedsRegistryWithUnknownFieldsAndModelChainWarning(t *testing.T) {
	root := t.TempDir()
	dir := writeConfig(t, root, "project", `
agents:
  architect:
    description: Architect
    aliases: [oracle]
    model: anthropic/claude-opus-4-7
    model_chain:
      - model: openai/gpt-5.5
        variant: high
    mystery_field: true
`)

	cfg, err := config.Load(config.Options{ProjectDir: dir})
	if err != nil {
		t.Fatalf("config Load returned error: %v", err)
	}
	reg, err := Build(cfg, Options{})
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	agent, ok := reg.Agent("architect")
	if !ok {
		t.Fatalf("architect was not loaded")
	}
	if len(agent.Models) != 1 || agent.Models[0].Model != "openai/gpt-5.5" {
		t.Fatalf("model_chain did not win: %#v", agent.Models)
	}
	if len(reg.Warnings) != 2 {
		t.Fatalf("warnings = %#v, want unknown field + model_chain warning", reg.Warnings)
	}
}

func writeConfig(t *testing.T, root, name, content string) string {
	t.Helper()
	dir := filepath.Join(root, name, ".omg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
