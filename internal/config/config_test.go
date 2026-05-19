package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesBuiltInUserProjectPrecedence(t *testing.T) {
	root := t.TempDir()
	builtIn := writeConfig(t, root, "builtin", `
agents:
  architect:
    description: Built-in architect
    category: advisor
    tools: [read]
`)
	user := writeConfig(t, root, "user", `
agents:
  architect:
    description: User architect
    tools: [read, grep]
`)
	project := writeConfig(t, root, "project", `
agents:
  architect:
    category: project-advisor
`)

	cfg, err := Load(Options{BuiltInDir: builtIn, UserDir: user, ProjectDir: project})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	agent := cfg.Agents["architect"]
	if agent.Description != "User architect" {
		t.Fatalf("description = %q, want user override", agent.Description)
	}
	if agent.Category != "project-advisor" {
		t.Fatalf("category = %q, want project override", agent.Category)
	}
	if got := agent.Tools; len(got) != 2 || got[0] != "read" || got[1] != "grep" {
		t.Fatalf("tools = %#v, want user tools preserved", got)
	}
}

func TestLoadReplacesListsAndSupportsExplicitAppend(t *testing.T) {
	root := t.TempDir()
	builtIn := writeConfig(t, root, "builtin", `
agents:
  explore:
    description: Explore
    tools: [read]
`)
	user := writeConfig(t, root, "user", `
agents:
  explore:
    tools: [read, grep]
`)
	project := writeConfig(t, root, "project", `
agents:
  explore:
    tools: [glob]
    tools_append: [lsp_diagnostics]
`)

	cfg, err := Load(Options{BuiltInDir: builtIn, UserDir: user, ProjectDir: project})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	want := []string{"glob", "lsp_diagnostics"}
	if got := cfg.Agents["explore"].Tools; !equalStrings(got, want) {
		t.Fatalf("tools = %#v, want %#v", got, want)
	}
}

func TestProjectDisabledShadowsUserDefinition(t *testing.T) {
	root := t.TempDir()
	user := writeConfig(t, root, "user", `
agents:
  experimental-agent:
    description: User experiment
    tools: [read]
skills:
  experimental-skill:
    description: User skill
    prompt: prompt.md
`)
	project := writeConfig(t, root, "project", `
agents:
  experimental-agent:
    disabled: true
skills:
  experimental-skill:
    disabled: true
`)

	cfg, err := Load(Options{UserDir: user, ProjectDir: project})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if !cfg.Agents["experimental-agent"].Disabled {
		t.Fatalf("project disabled did not shadow user agent: %#v", cfg.Agents["experimental-agent"])
	}
	if !cfg.Skills["experimental-skill"].Disabled {
		t.Fatalf("project disabled did not shadow user skill: %#v", cfg.Skills["experimental-skill"])
	}
}

func TestLoadReturnsErrorForInvalidConfig(t *testing.T) {
	root := t.TempDir()
	project := writeConfig(t, root, "project", `
agents:
  broken:
    tools: "read"
`)

	_, err := Load(Options{ProjectDir: project})
	if err == nil {
		t.Fatalf("expected invalid config error")
	}
}

func TestLoadMergesDatabaseStorageConfig(t *testing.T) {
	root := t.TempDir()
	user := writeConfig(t, root, "user", `
storage:
  database:
    path: /tmp/user.db
`)
	project := writeConfig(t, root, "project", `
storage:
  database:
    path: /tmp/project.db
`)

	cfg, err := Load(Options{UserDir: user, ProjectDir: project})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if cfg.Storage.Database.Path != "/tmp/project.db" {
		t.Fatalf("database path = %q", cfg.Storage.Database.Path)
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

func TestLoad_WorkerConfig_ProjectLevel(t *testing.T) {
	root := t.TempDir()
	project := writeConfig(t, root, "project", `
workers:
  pi:
    kind: pi
    binary_path: /opt/pi/bin/pi
`)

	cfg, err := Load(Options{ProjectDir: project})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	got := cfg.Workers["pi"]
	if got.Kind != "pi" {
		t.Fatalf("Kind = %q, want %q", got.Kind, "pi")
	}
	if got.BinaryPath != "/opt/pi/bin/pi" {
		t.Fatalf("BinaryPath = %q, want %q", got.BinaryPath, "/opt/pi/bin/pi")
	}
}

func TestLoad_WorkerConfig_EmptyOK(t *testing.T) {
	root := t.TempDir()
	project := writeConfig(t, root, "project", `agents: {}`)

	cfg, err := Load(Options{ProjectDir: project})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if cfg.Workers == nil {
		t.Fatalf("Workers should be non-nil empty map")
	}
	if len(cfg.Workers) != 0 {
		t.Fatalf("Workers should be empty, got %d entries", len(cfg.Workers))
	}
}

func TestLoad_WorkerConfig_LayeredMerge(t *testing.T) {
	root := t.TempDir()
	user := writeConfig(t, root, "user", `
workers:
  pi:
    kind: pi
    binary_path: /usr/local/bin/pi
`)
	project := writeConfig(t, root, "project", `
workers:
  pi:
    binary_path: /opt/pi/bin/pi
`)

	cfg, err := Load(Options{UserDir: user, ProjectDir: project})
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	got := cfg.Workers["pi"]
	if got.Kind != "pi" {
		t.Fatalf("Kind = %q, want %q (preserved from user)", got.Kind, "pi")
	}
	if got.BinaryPath != "/opt/pi/bin/pi" {
		t.Fatalf("BinaryPath = %q, want %q (project override)", got.BinaryPath, "/opt/pi/bin/pi")
	}
}
