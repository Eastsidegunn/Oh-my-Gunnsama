package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSkillParsesYAMLAndPromptWithoutStartingMCP(t *testing.T) {
	dir := writeSkill(t, `
name: ast-search
description: "AST-based code pattern search"
triggers:
  - pattern: "ast|structure|pattern search"
    priority: 10
prompt: prompt.md
mcp:
  server: ast-grep-server
  tools: [ast_search]
  env_inherit: [PATH]
`, "Use AST search")

	skill, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error: %v", err)
	}
	if skill.Name != "ast-search" || skill.Description == "" || skill.PromptText != "Use AST search" {
		t.Fatalf("skill mismatch: %#v", skill)
	}
	if len(skill.Triggers) != 1 || skill.Triggers[0].Pattern != "ast|structure|pattern search" || skill.Triggers[0].Priority != 10 {
		t.Fatalf("triggers mismatch: %#v", skill.Triggers)
	}
	if skill.MCP == nil || skill.MCP.Server != "ast-grep-server" || len(skill.MCP.Tools) != 1 || skill.MCP.EnvInherit[0] != "PATH" {
		t.Fatalf("mcp mismatch: %#v", skill.MCP)
	}
}

func TestLoadSkillRejectsAgentAndModelFields(t *testing.T) {
	dir := writeSkill(t, `
name: bad-skill
description: Bad
agent: executor
model: openai/gpt-5.5
`, "")

	_, err := Load(dir)
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("error = %v, want rejected agent/model fields", err)
	}
}

func writeSkill(t *testing.T, yamlText, prompt string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "skill.yaml"), []byte(yamlText), 0o644); err != nil {
		t.Fatalf("write skill.yaml: %v", err)
	}
	if prompt != "" {
		if err := os.WriteFile(filepath.Join(dir, "prompt.md"), []byte(prompt), 0o644); err != nil {
			t.Fatalf("write prompt.md: %v", err)
		}
	}
	return dir
}
