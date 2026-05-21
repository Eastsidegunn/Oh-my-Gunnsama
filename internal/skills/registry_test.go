package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSkillDir(t *testing.T, base, name, desc string) string {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	content := "name: " + name + "\ndescription: " + desc + "\n"
	if err := os.WriteFile(filepath.Join(dir, SkillFilename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestLoadAll_EmptyScopesReturnsEmpty(t *testing.T) {
	reg, err := LoadAll(nil)
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(reg.Skills) != 0 {
		t.Errorf("expected empty registry, got %d skills", len(reg.Skills))
	}
}

func TestLoadAll_MissingDirIsSkipped(t *testing.T) {
	reg, err := LoadAll([]ScopePath{{Scope: ScopeUser, Dir: "/nonexistent/path/xyz"}})
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if len(reg.Skills) != 0 {
		t.Errorf("expected empty registry, got %d", len(reg.Skills))
	}
}

func TestLoadAll_LoadsSkillsFromDir(t *testing.T) {
	base := t.TempDir()
	writeSkillDir(t, base, "refactor", "Refactoring skill")
	writeSkillDir(t, base, "review", "Review skill")

	reg, err := LoadAll([]ScopePath{{Scope: ScopeUser, Dir: base}})
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if len(reg.Skills) != 2 {
		t.Errorf("expected 2 skills, got %d", len(reg.Skills))
	}
	if _, ok := reg.Get("refactor"); !ok {
		t.Error("refactor skill missing")
	}
}

func TestLoadAll_ProjectScopeWinsOverUser(t *testing.T) {
	projectBase := t.TempDir()
	userBase := t.TempDir()
	writeSkillDir(t, projectBase, "refactor", "Project version")
	writeSkillDir(t, userBase, "refactor", "User version")

	reg, err := LoadAll([]ScopePath{
		{Scope: ScopeProject, Dir: projectBase},
		{Scope: ScopeUser, Dir: userBase},
	})
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	s, ok := reg.Get("refactor")
	if !ok {
		t.Fatal("refactor missing")
	}
	if s.Description != "Project version" {
		t.Errorf("description=%q, want 'Project version' (project scope wins)", s.Description)
	}
}

func TestLoadAll_BuiltinScopeLowestPriority(t *testing.T) {
	builtinBase := t.TempDir()
	userBase := t.TempDir()
	writeSkillDir(t, builtinBase, "git-master", "Builtin version")
	writeSkillDir(t, userBase, "git-master", "User override")

	reg, err := LoadAll([]ScopePath{
		{Scope: ScopeBuiltin, Dir: builtinBase},
		{Scope: ScopeUser, Dir: userBase},
	})
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	s, _ := reg.Get("git-master")
	if s.Description != "User override" {
		t.Errorf("description=%q, want 'User override' (user beats builtin)", s.Description)
	}
}

func TestLoadAll_InvalidSkillDirSkipped(t *testing.T) {
	base := t.TempDir()
	writeSkillDir(t, base, "valid", "Valid skill")
	badDir := filepath.Join(base, "bad")
	os.MkdirAll(badDir, 0755)
	os.WriteFile(filepath.Join(badDir, SkillFilename), []byte("name: \ndescription: \n"), 0644)

	reg, err := LoadAll([]ScopePath{{Scope: ScopeUser, Dir: base}})
	if err != nil {
		t.Fatalf("LoadAll error: %v", err)
	}
	if _, ok := reg.Get("valid"); !ok {
		t.Error("valid skill should be loaded")
	}
	if _, ok := reg.Get("bad"); ok {
		t.Error("invalid skill should be skipped")
	}
}

func TestRegistry_All_ReturnsSorted(t *testing.T) {
	base := t.TempDir()
	writeSkillDir(t, base, "zebra", "Z")
	writeSkillDir(t, base, "alpha", "A")
	writeSkillDir(t, base, "mango", "M")

	reg, _ := LoadAll([]ScopePath{{Scope: ScopeUser, Dir: base}})
	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("expected 3, got %d", len(all))
	}
	if all[0].Name != "alpha" || all[1].Name != "mango" || all[2].Name != "zebra" {
		t.Errorf("not sorted: %v", []string{all[0].Name, all[1].Name, all[2].Name})
	}
}
