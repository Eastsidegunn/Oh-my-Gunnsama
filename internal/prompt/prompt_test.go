package prompt

import (
	"strings"
	"testing"

	"oh-my-gunnsama/internal/registry"
	"oh-my-gunnsama/internal/route"
)

func TestBuildPromptAssemblesSectionsInOrder(t *testing.T) {
	input := BuildPromptInput{
		Agent:   registry.Agent{Name: "architect", Description: "Design deep modules"},
		Model:   ModelInfo{Model: "openai/gpt-5.5", Family: "gpt"},
		Project: ProjectContext{Rules: "Project rules"},
		Route:   route.RouteResult{Procedure: route.ProcedureReview, Domain: "architecture", Agent: "architect", Confidence: route.ConfidenceHigh},
		Sections: []SectionSpec{
			{Name: "identity", Source: "dynamic", Content: "IDENTITY", Required: true, Priority: 100},
			{Name: "safety", Source: "template/safety.md", Content: "SAFETY", Required: true, Priority: 90},
			{Name: "project", Source: "project", Content: "PROJECT", Required: false, Priority: 10},
		},
	}

	out, err := BuildPrompt(input)
	if err != nil {
		t.Fatalf("BuildPrompt returned error: %v", err)
	}
	if out.SystemPrompt != "IDENTITY\n\nSAFETY\n\nPROJECT" {
		t.Fatalf("SystemPrompt = %q", out.SystemPrompt)
	}
	if len(out.Sections) != 3 {
		t.Fatalf("Sections = %#v", out.Sections)
	}
	for i, name := range []string{"identity", "safety", "project"} {
		if out.Sections[i].Name != name {
			t.Fatalf("section[%d].Name = %q, want %q", i, out.Sections[i].Name, name)
		}
	}
}

func TestBuildPromptErrorsWhenRequiredSectionCannotFitBudget(t *testing.T) {
	_, err := BuildPrompt(BuildPromptInput{
		Agent:  registry.Agent{Name: "architect", Description: "Design"},
		Model:  ModelInfo{Model: "openai/gpt-5.5"},
		Route:  route.RouteResult{Procedure: route.ProcedureReview, Agent: "architect", Confidence: route.ConfidenceHigh},
		Budget: TokenBudget{MaxTokens: 1},
		Sections: []SectionSpec{
			{Name: "identity", Source: "dynamic", Content: "too many words", Required: true, Priority: 100},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "required section") {
		t.Fatalf("error = %v, want required section budget error", err)
	}
}

func TestBuildPromptOmitsOptionalSectionsByLowestPriorityAndWarns(t *testing.T) {
	out, err := BuildPrompt(BuildPromptInput{
		Agent:  registry.Agent{Name: "architect", Description: "Design"},
		Model:  ModelInfo{Model: "openai/gpt-5.5"},
		Route:  route.RouteResult{Procedure: route.ProcedureReview, Agent: "architect", Confidence: route.ConfidenceHigh},
		Budget: TokenBudget{MaxTokens: 4},
		Sections: []SectionSpec{
			{Name: "identity", Source: "dynamic", Content: "identity one", Required: true, Priority: 100},
			{Name: "low", Source: "optional-low", Content: "low optional", Required: false, Priority: 1},
			{Name: "high", Source: "optional-high", Content: "high optional", Required: false, Priority: 50},
		},
	})
	if err != nil {
		t.Fatalf("BuildPrompt returned error: %v", err)
	}
	if out.SystemPrompt != "identity one\n\nhigh optional" {
		t.Fatalf("SystemPrompt = %q", out.SystemPrompt)
	}
	if len(out.OmittedSections) != 1 || out.OmittedSections[0].Name != "low" {
		t.Fatalf("OmittedSections = %#v", out.OmittedSections)
	}
	if len(out.Warnings) != 1 || !strings.Contains(out.Warnings[0], "low") {
		t.Fatalf("Warnings = %#v", out.Warnings)
	}
	if out.TokenEstimate != 4 {
		t.Fatalf("TokenEstimate = %d, want 4", out.TokenEstimate)
	}
}

func TestBuildPromptReturnsAccurateSectionMetadata(t *testing.T) {
	ctx := &PromptContextSnapshot{Summary: "Prior context"}
	out, err := BuildPrompt(BuildPromptInput{
		Agent:   registry.Agent{Name: "writer", Description: "Writes docs"},
		Model:   ModelInfo{Model: "anthropic/claude-opus-4-7", Family: "claude"},
		Project: ProjectContext{Rules: "No slop"},
		Context: ctx,
		Route:   route.RouteResult{Procedure: route.ProcedureExecute, Agent: "writer", Confidence: route.ConfidenceMedium},
		Flags:   map[string]string{"mode": "test"},
		Sections: []SectionSpec{
			{Name: "identity", Source: "dynamic", Content: "hello world", Required: true, Priority: 10},
		},
	})
	if err != nil {
		t.Fatalf("BuildPrompt returned error: %v", err)
	}
	if len(out.Sections) != 1 {
		t.Fatalf("Sections = %#v", out.Sections)
	}
	section := out.Sections[0]
	if section.Name != "identity" || section.Source != "dynamic" || section.Tokens != 2 || !section.Required || section.Priority != 10 {
		t.Fatalf("section metadata mismatch: %#v", section)
	}
	if out.TokenEstimate != 2 {
		t.Fatalf("TokenEstimate = %d, want 2", out.TokenEstimate)
	}
}
