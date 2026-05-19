package prompt

import (
	"fmt"
	"sort"
	"strings"

	"oh-my-gunnsama/internal/registry"
	"oh-my-gunnsama/internal/route"
)

type BuildPromptInput struct {
	Agent    registry.Agent
	Model    ModelInfo
	Project  ProjectContext
	Context  *PromptContextSnapshot
	Route    route.RouteResult
	Flags    map[string]string
	Budget   TokenBudget
	Sections []SectionSpec
}

type ModelInfo struct {
	Model  string
	Family string
}

type ProjectContext struct {
	Rules string
}

type PromptContextSnapshot struct {
	Summary string
}

type TokenBudget struct {
	MaxTokens int
}

type SectionSpec struct {
	Name     string
	Source   string
	Content  string
	Required bool
	Priority int
}

type BuildPromptOutput struct {
	SystemPrompt    string
	Sections        []PromptSection
	OmittedSections []PromptSection
	Warnings        []string
	TokenEstimate   int
}

type PromptSection struct {
	Name     string
	Source   string
	Tokens   int
	Required bool
	Priority int
}

func BuildPrompt(input BuildPromptInput) (BuildPromptOutput, error) {
	selected, omitted, warnings, err := selectSections(input.Sections, input.Budget)
	if err != nil {
		return BuildPromptOutput{}, err
	}

	parts := make([]string, 0, len(selected))
	sections := make([]PromptSection, 0, len(selected))
	totalTokens := 0
	for _, section := range selected {
		content := strings.TrimSpace(section.Content)
		if content == "" {
			continue
		}
		parts = append(parts, content)
		metadata := metadataFor(section)
		sections = append(sections, metadata)
		totalTokens += metadata.Tokens
	}

	return BuildPromptOutput{
		SystemPrompt:    strings.Join(parts, "\n\n"),
		Sections:        sections,
		OmittedSections: omitted,
		Warnings:        warnings,
		TokenEstimate:   totalTokens,
	}, nil
}

func selectSections(sections []SectionSpec, budget TokenBudget) ([]SectionSpec, []PromptSection, []string, error) {
	selected := append([]SectionSpec(nil), sections...)
	maxTokens := budget.MaxTokens
	if maxTokens <= 0 {
		return selected, nil, nil, nil
	}

	requiredTokens := 0
	for _, section := range selected {
		if section.Required {
			requiredTokens += estimateTokens(section.Content)
		}
	}
	if requiredTokens > maxTokens {
		return nil, nil, nil, fmt.Errorf("required section token budget exceeded: required=%d max=%d", requiredTokens, maxTokens)
	}

	omitted := []PromptSection{}
	warnings := []string{}
	for totalTokens(selected) > maxTokens {
		idx := lowestPriorityOptionalIndex(selected)
		if idx < 0 {
			return nil, nil, nil, fmt.Errorf("required section token budget exceeded: total=%d max=%d", totalTokens(selected), maxTokens)
		}
		section := selected[idx]
		omitted = append(omitted, metadataFor(section))
		warnings = append(warnings, fmt.Sprintf("omitted optional section %q due to token budget", section.Name))
		selected = append(selected[:idx], selected[idx+1:]...)
	}
	return selected, omitted, warnings, nil
}

func lowestPriorityOptionalIndex(sections []SectionSpec) int {
	candidates := []int{}
	for i, section := range sections {
		if !section.Required {
			candidates = append(candidates, i)
		}
	}
	if len(candidates) == 0 {
		return -1
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		left := sections[candidates[i]]
		right := sections[candidates[j]]
		if left.Priority != right.Priority {
			return left.Priority < right.Priority
		}
		return candidates[i] < candidates[j]
	})
	return candidates[0]
}

func totalTokens(sections []SectionSpec) int {
	total := 0
	for _, section := range sections {
		total += estimateTokens(section.Content)
	}
	return total
}

func metadataFor(section SectionSpec) PromptSection {
	return PromptSection{
		Name:     section.Name,
		Source:   section.Source,
		Tokens:   estimateTokens(section.Content),
		Required: section.Required,
		Priority: section.Priority,
	}
}

func estimateTokens(content string) int {
	return len(strings.Fields(content))
}
