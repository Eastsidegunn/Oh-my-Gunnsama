package prompt

import (
	"context"
	"errors"
	"strings"
	"testing"

	"oh-my-gunnsama/internal/registry"
	"oh-my-gunnsama/internal/skills"
)

func emptyInput() DynamicInput {
	return DynamicInput{
		Agent:    registry.Agent{Name: "test"},
		Registry: registry.Registry{Agents: map[string]registry.Agent{}},
		Skills:   []skills.Skill{},
	}
}

func TestBuildDynamic_AssemblesInPriorityOrder(t *testing.T) {
	sections := []Section{
		StaticSection{SectionName: "c", SectionPriority: 30, Content: "third"},
		StaticSection{SectionName: "a", SectionPriority: 10, Content: "first"},
		StaticSection{SectionName: "b", SectionPriority: 20, Content: "second"},
	}
	out, err := BuildDynamic(context.Background(), sections, emptyInput())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	parts := strings.Split(out.SystemPrompt, "\n\n")
	if len(parts) != 3 || parts[0] != "first" || parts[1] != "second" || parts[2] != "third" {
		t.Errorf("wrong order: %v", parts)
	}
}

func TestBuildDynamic_SkipsEmptySections(t *testing.T) {
	sections := []Section{
		StaticSection{SectionName: "empty", SectionPriority: 10, Content: "   "},
		StaticSection{SectionName: "real", SectionPriority: 20, Content: "content"},
	}
	out, err := BuildDynamic(context.Background(), sections, emptyInput())
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if out.SystemPrompt != "content" {
		t.Errorf("prompt=%q, want 'content'", out.SystemPrompt)
	}
}

func TestBuildDynamic_SectionErrorBecomesWarning(t *testing.T) {
	sections := []Section{
		FuncSection{SectionName: "bad", SectionPriority: 10, Fn: func(_ context.Context, _ DynamicInput) (string, error) {
			return "", errors.New("build failed")
		}},
		StaticSection{SectionName: "good", SectionPriority: 20, Content: "ok"},
	}
	out, err := BuildDynamic(context.Background(), sections, emptyInput())
	if err != nil {
		t.Fatalf("error should not propagate: %v", err)
	}
	if out.SystemPrompt != "ok" {
		t.Errorf("prompt=%q, want 'ok'", out.SystemPrompt)
	}
	if len(out.Warnings) == 0 || !strings.Contains(out.Warnings[0], "build failed") {
		t.Errorf("warnings=%v, want build failed", out.Warnings)
	}
}

func TestBuildDynamic_FuncSectionReceivesInput(t *testing.T) {
	var gotAgent string
	sections := []Section{
		FuncSection{SectionName: "identity", SectionPriority: 10, Fn: func(_ context.Context, in DynamicInput) (string, error) {
			gotAgent = in.Agent.Name
			return "agent: " + in.Agent.Name, nil
		}},
	}
	in := emptyInput()
	in.Agent.Name = "sisyphus"
	out, _ := BuildDynamic(context.Background(), sections, in)
	if gotAgent != "sisyphus" {
		t.Errorf("agent not passed to section, got %q", gotAgent)
	}
	if !strings.Contains(out.SystemPrompt, "sisyphus") {
		t.Errorf("prompt=%q, want sisyphus", out.SystemPrompt)
	}
}

func TestBuildDynamic_SkillsAvailableInInput(t *testing.T) {
	var gotSkills []string
	sections := []Section{
		FuncSection{SectionName: "skills", SectionPriority: 10, Fn: func(_ context.Context, in DynamicInput) (string, error) {
			for _, s := range in.Skills {
				gotSkills = append(gotSkills, s.Name)
			}
			return "skills listed", nil
		}},
	}
	in := emptyInput()
	in.Skills = []skills.Skill{{Name: "refactor"}, {Name: "review-work"}}
	BuildDynamic(context.Background(), sections, in)
	if len(gotSkills) != 2 || gotSkills[0] != "refactor" {
		t.Errorf("skills not passed: %v", gotSkills)
	}
}

func TestBuildDynamic_OtherAgentsAvailableViaRegistry(t *testing.T) {
	var sawOracle bool
	sections := []Section{
		FuncSection{SectionName: "delegation", SectionPriority: 10, Fn: func(_ context.Context, in DynamicInput) (string, error) {
			_, sawOracle = in.Registry.Agent("oracle")
			return "delegation table", nil
		}},
	}
	in := emptyInput()
	in.Registry.Agents["oracle"] = registry.Agent{Name: "oracle"}
	BuildDynamic(context.Background(), sections, in)
	if !sawOracle {
		t.Error("oracle should be visible via Registry in DynamicInput")
	}
}

func TestStaticSection_Interface(t *testing.T) {
	var _ Section = StaticSection{}
	var _ Section = FuncSection{}
}
