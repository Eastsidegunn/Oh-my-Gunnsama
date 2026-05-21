package prompt

import (
	"context"
	"sort"
	"strings"

	"oh-my-gunnsama/internal/registry"
	"oh-my-gunnsama/internal/skills"
)

type DynamicInput struct {
	Agent      registry.Agent
	Model      ModelInfo
	Registry   registry.Registry
	Skills     []skills.Skill
	Tools      []string
	Categories []string
}

type Section interface {
	Name() string
	Priority() int
	Build(ctx context.Context, in DynamicInput) (string, error)
}

type StaticSection struct {
	SectionName     string
	SectionPriority int
	Content         string
}

func (s StaticSection) Name() string  { return s.SectionName }
func (s StaticSection) Priority() int { return s.SectionPriority }
func (s StaticSection) Build(_ context.Context, _ DynamicInput) (string, error) {
	return s.Content, nil
}

type FuncSection struct {
	SectionName     string
	SectionPriority int
	Fn              func(context.Context, DynamicInput) (string, error)
}

func (f FuncSection) Name() string  { return f.SectionName }
func (f FuncSection) Priority() int { return f.SectionPriority }
func (f FuncSection) Build(ctx context.Context, in DynamicInput) (string, error) {
	return f.Fn(ctx, in)
}

type DynamicOutput struct {
	SystemPrompt string
	Warnings     []string
}

func BuildDynamic(ctx context.Context, sections []Section, in DynamicInput) (DynamicOutput, error) {
	sorted := make([]Section, len(sections))
	copy(sorted, sections)
	sort.SliceStable(sorted, func(i, j int) bool {
		return sorted[i].Priority() < sorted[j].Priority()
	})

	var parts []string
	var warnings []string
	for _, sec := range sorted {
		content, err := sec.Build(ctx, in)
		if err != nil {
			warnings = append(warnings, "section "+sec.Name()+": "+err.Error())
			continue
		}
		content = strings.TrimSpace(content)
		if content != "" {
			parts = append(parts, content)
		}
	}
	return DynamicOutput{
		SystemPrompt: strings.Join(parts, "\n\n"),
		Warnings:     warnings,
	}, nil
}
