package registry

import (
	"fmt"
	"sort"

	"oh-my-gunnsama/internal/config"
)

type Options struct {
	Strict bool
}

type Registry struct {
	Agents   map[string]Agent
	Aliases  map[string]string
	Warnings []string
}

type Agent struct {
	Name        string
	Description string
	Aliases     []string
	Category    string
	Autonomy    string
	Cost        string
	Mode        string
	Models      []ModelCandidate
	Tools       ToolPolicy
	Triggers    []Trigger
	UseWhen     []string
	AvoidWhen   []string
	Source      string
}

type ModelCandidate struct {
	Model           string
	Variant         string
	ReasoningEffort string
	Params          map[string]any
}

type Trigger struct {
	Domain   string
	Pattern  string
	Priority int
}

type ToolMode string

const (
	ToolModeDefault   ToolMode = "default"
	ToolModeAllowlist ToolMode = "allowlist"
	ToolModeAllExcept ToolMode = "all_except"
)

type ToolPolicy struct {
	Mode   ToolMode
	Allow  []string
	Except []string
}

func Build(cfg config.Config, options Options) (Registry, error) {
	reg := Registry{
		Agents:  map[string]Agent{},
		Aliases: map[string]string{},
	}

	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		definition := cfg.Agents[name]
		if definition.Disabled {
			continue
		}
		if definition.Name == "" {
			return Registry{}, fmt.Errorf("agent %q missing name", name)
		}
		if definition.Description == "" {
			return Registry{}, fmt.Errorf("agent %q missing description", definition.Name)
		}
		if len(definition.UnknownFields) > 0 {
			for _, field := range definition.UnknownFields {
				warning := fmt.Sprintf("agent %q unknown field %q", definition.Name, field)
				if options.Strict {
					return Registry{}, fmt.Errorf("unknown field: %s", warning)
				}
				reg.Warnings = append(reg.Warnings, warning)
			}
		}

		agent, warnings, err := normalizeAgent(definition)
		if err != nil {
			return Registry{}, err
		}
		if options.Strict && len(warnings) > 0 {
			return Registry{}, fmt.Errorf("strict warning: %s", warnings[0])
		}
		reg.Warnings = append(reg.Warnings, warnings...)
		reg.Agents[agent.Name] = agent
	}

	for _, name := range sortedAgentNames(reg.Agents) {
		agent := reg.Agents[name]
		for _, alias := range agent.Aliases {
			if alias == "" {
				continue
			}
			if existing, ok := reg.Aliases[alias]; ok && existing != agent.Name {
				return Registry{}, fmt.Errorf("alias collision: %q maps to both %q and %q", alias, existing, agent.Name)
			}
			if _, existsAsAgent := reg.Agents[alias]; existsAsAgent && alias != agent.Name {
				return Registry{}, fmt.Errorf("alias collision: %q conflicts with canonical agent name", alias)
			}
			reg.Aliases[alias] = agent.Name
		}
	}

	return reg, nil
}

func (r Registry) Agent(name string) (Agent, bool) {
	agent, ok := r.Agents[name]
	return agent, ok
}

func (r Registry) ResolveAlias(alias string) (string, bool) {
	name, ok := r.Aliases[alias]
	return name, ok
}

func normalizeAgent(definition config.AgentDefinition) (Agent, []string, error) {
	tools, err := normalizeTools(definition)
	if err != nil {
		return Agent{}, nil, fmt.Errorf("agent %q: %w", definition.Name, err)
	}
	models, warnings := normalizeModels(definition)
	return Agent{
		Name:        definition.Name,
		Description: definition.Description,
		Aliases:     cloneStrings(definition.Aliases),
		Category:    definition.Category,
		Autonomy:    definition.Autonomy,
		Cost:        definition.Cost,
		Mode:        definition.Mode,
		Models:      models,
		Tools:       tools,
		Triggers:    cloneTriggers(definition.Triggers),
		UseWhen:     cloneStrings(definition.UseWhen),
		AvoidWhen:   cloneStrings(definition.AvoidWhen),
		Source:      definition.Source,
	}, warnings, nil
}

func normalizeTools(definition config.AgentDefinition) (ToolPolicy, error) {
	if definition.ToolsAll {
		return ToolPolicy{Mode: ToolModeAllExcept, Except: cloneStrings(definition.ToolsExcept)}, nil
	}
	if len(definition.ToolsExcept) > 0 {
		return ToolPolicy{}, fmt.Errorf("tools_except is valid only when tools: all")
	}
	if definition.Tools != nil {
		return ToolPolicy{Mode: ToolModeAllowlist, Allow: cloneStrings(definition.Tools)}, nil
	}
	return ToolPolicy{Mode: ToolModeDefault}, nil
}

func normalizeModels(definition config.AgentDefinition) ([]ModelCandidate, []string) {
	if len(definition.ModelChain) > 0 {
		models := make([]ModelCandidate, len(definition.ModelChain))
		for i, candidate := range definition.ModelChain {
			models[i] = ModelCandidate{
				Model:           candidate.Model,
				Variant:         candidate.Variant,
				ReasoningEffort: candidate.ReasoningEffort,
				Params:          candidate.Params,
			}
		}
		if definition.Model != "" {
			return models, []string{fmt.Sprintf("agent %q has both model and model_chain; model_chain wins", definition.Name)}
		}
		return models, nil
	}
	if definition.Model != "" {
		return []ModelCandidate{{Model: definition.Model}}, nil
	}
	return nil, nil
}

func sortedAgentNames(agents map[string]Agent) []string {
	names := make([]string, 0, len(agents))
	for name := range agents {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneTriggers(values []config.TriggerDefinition) []Trigger {
	if values == nil {
		return nil
	}
	cloned := make([]Trigger, len(values))
	for i, value := range values {
		cloned[i] = Trigger{Domain: value.Domain, Pattern: value.Pattern, Priority: value.Priority}
	}
	return cloned
}
