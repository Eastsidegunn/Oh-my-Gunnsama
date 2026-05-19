package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const ConfigFilename = "config.yaml"

type Options struct {
	BuiltInDir string
	UserDir    string
	ProjectDir string
}

type Config struct {
	Agents  map[string]AgentDefinition
	Skills  map[string]SkillDefinition
	Storage StorageConfig
	Workers map[string]WorkerConfig
}

type StorageConfig struct {
	Database DatabaseStorageConfig
}

type DatabaseStorageConfig struct {
	Path string
}

type WorkerConfig struct {
	Kind       string
	BinaryPath string
}

type AgentDefinition struct {
	Name          string
	Description   string
	Aliases       []string
	Category      string
	Autonomy      string
	Cost          string
	Mode          string
	Model         string
	ModelChain    []ModelCandidate
	Tools         []string
	ToolsAll      bool
	ToolsExcept   []string
	Triggers      []TriggerDefinition
	UseWhen       []string
	AvoidWhen     []string
	Disabled      bool
	UnknownFields []string
	Source        string
}

type ModelCandidate struct {
	Model           string         `yaml:"model"`
	Variant         string         `yaml:"variant"`
	ReasoningEffort string         `yaml:"reasoningEffort"`
	Params          map[string]any `yaml:"params"`
}

type TriggerDefinition struct {
	Domain   string `yaml:"domain"`
	Pattern  string `yaml:"pattern"`
	Priority int    `yaml:"priority"`
}

type SkillDefinition struct {
	Name        string
	Description string
	Prompt      string
	Disabled    bool
	Source      string
}

type configPatch struct {
	Agents  map[string]agentPatch  `yaml:"agents"`
	Skills  map[string]skillPatch  `yaml:"skills"`
	Storage storagePatch           `yaml:"storage"`
	Workers map[string]workerPatch `yaml:"workers"`
}

type storagePatch struct {
	Database databasePatch `yaml:"database"`
}

type databasePatch struct {
	Path *string `yaml:"path"`
}

type workerPatch struct {
	Kind       *string `yaml:"kind"`
	BinaryPath *string `yaml:"binary_path"`
}

type agentPatch struct {
	Description   *string
	Aliases       []string
	Category      *string
	Autonomy      *string
	Cost          *string
	Mode          *string
	Model         *string
	ModelChain    []ModelCandidate
	Tools         []string
	ToolsSet      bool
	ToolsAll      bool
	ToolsAppend   []string
	ToolsExcept   []string
	Triggers      []TriggerDefinition
	UseWhen       []string
	AvoidWhen     []string
	Disabled      *bool
	UnknownFields []string
}

type skillPatch struct {
	Description *string `yaml:"description"`
	Prompt      *string `yaml:"prompt"`
	Disabled    *bool   `yaml:"disabled"`
}

func Load(options Options) (Config, error) {
	cfg := Config{
		Agents:  map[string]AgentDefinition{},
		Skills:  map[string]SkillDefinition{},
		Workers: map[string]WorkerConfig{},
	}

	for _, source := range []struct {
		name string
		dir  string
	}{
		{name: "built-in", dir: options.BuiltInDir},
		{name: "user", dir: options.UserDir},
		{name: "project", dir: options.ProjectDir},
	} {
		if source.dir == "" {
			continue
		}
		patch, err := loadPatch(source.dir)
		if err != nil {
			return Config{}, fmt.Errorf("load %s config: %w", source.name, err)
		}
		cfg.applyPatch(patch, source.name)
	}

	return cfg, nil
}

func loadPatch(dir string) (configPatch, error) {
	path := filepath.Join(dir, ConfigFilename)
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return configPatch{}, nil
		}
		return configPatch{}, err
	}

	var patch configPatch
	if err := yaml.Unmarshal(content, &patch); err != nil {
		return configPatch{}, err
	}
	return patch, nil
}

func (cfg *Config) applyPatch(patch configPatch, source string) {
	if patch.Storage.Database.Path != nil {
		cfg.Storage.Database.Path = *patch.Storage.Database.Path
	}

	for name, agentPatch := range patch.Agents {
		agent := cfg.Agents[name]
		agent.Name = name
		agent.Source = source
		if agentPatch.Description != nil {
			agent.Description = *agentPatch.Description
		}
		if agentPatch.Aliases != nil {
			agent.Aliases = cloneStrings(agentPatch.Aliases)
		}
		if agentPatch.Category != nil {
			agent.Category = *agentPatch.Category
		}
		if agentPatch.Autonomy != nil {
			agent.Autonomy = *agentPatch.Autonomy
		}
		if agentPatch.Cost != nil {
			agent.Cost = *agentPatch.Cost
		}
		if agentPatch.Mode != nil {
			agent.Mode = *agentPatch.Mode
		}
		if agentPatch.Model != nil {
			agent.Model = *agentPatch.Model
		}
		if agentPatch.ModelChain != nil {
			agent.ModelChain = cloneModels(agentPatch.ModelChain)
		}
		if agentPatch.ToolsSet {
			agent.Tools = cloneStrings(agentPatch.Tools)
			agent.ToolsAll = agentPatch.ToolsAll
		}
		if agentPatch.ToolsAppend != nil {
			agent.Tools = append(agent.Tools, agentPatch.ToolsAppend...)
		}
		if agentPatch.ToolsExcept != nil {
			agent.ToolsExcept = cloneStrings(agentPatch.ToolsExcept)
		}
		if agentPatch.Triggers != nil {
			agent.Triggers = cloneTriggers(agentPatch.Triggers)
		}
		if agentPatch.UseWhen != nil {
			agent.UseWhen = cloneStrings(agentPatch.UseWhen)
		}
		if agentPatch.AvoidWhen != nil {
			agent.AvoidWhen = cloneStrings(agentPatch.AvoidWhen)
		}
		if agentPatch.Disabled != nil {
			agent.Disabled = *agentPatch.Disabled
		}
		if agentPatch.UnknownFields != nil {
			agent.UnknownFields = cloneStrings(agentPatch.UnknownFields)
		}
		cfg.Agents[name] = agent
	}

	for name, skillPatch := range patch.Skills {
		skill := cfg.Skills[name]
		skill.Name = name
		skill.Source = source
		if skillPatch.Description != nil {
			skill.Description = *skillPatch.Description
		}
		if skillPatch.Prompt != nil {
			skill.Prompt = *skillPatch.Prompt
		}
		if skillPatch.Disabled != nil {
			skill.Disabled = *skillPatch.Disabled
		}
		cfg.Skills[name] = skill
	}

	for name, wp := range patch.Workers {
		worker := cfg.Workers[name]
		if wp.Kind != nil {
			worker.Kind = *wp.Kind
		}
		if wp.BinaryPath != nil {
			worker.BinaryPath = *wp.BinaryPath
		}
		cfg.Workers[name] = worker
	}
}

func (p *agentPatch) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("agent definition must be a mapping")
	}
	known := map[string]bool{
		"description": true, "aliases": true, "category": true, "autonomy": true,
		"cost": true, "mode": true, "model": true, "model_chain": true,
		"tools": true, "tools_append": true, "tools_except": true, "triggers": true,
		"useWhen": true, "avoidWhen": true, "disabled": true,
	}
	for i := 0; i < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch key {
		case "description":
			var out string
			if err := val.Decode(&out); err != nil {
				return fmt.Errorf("description: %w", err)
			}
			p.Description = &out
		case "aliases":
			if err := val.Decode(&p.Aliases); err != nil {
				return fmt.Errorf("aliases: %w", err)
			}
		case "category":
			var out string
			if err := val.Decode(&out); err != nil {
				return fmt.Errorf("category: %w", err)
			}
			p.Category = &out
		case "autonomy":
			var out string
			if err := val.Decode(&out); err != nil {
				return fmt.Errorf("autonomy: %w", err)
			}
			p.Autonomy = &out
		case "cost":
			var out string
			if err := val.Decode(&out); err != nil {
				return fmt.Errorf("cost: %w", err)
			}
			p.Cost = &out
		case "mode":
			var out string
			if err := val.Decode(&out); err != nil {
				return fmt.Errorf("mode: %w", err)
			}
			p.Mode = &out
		case "model":
			var out string
			if err := val.Decode(&out); err != nil {
				return fmt.Errorf("model: %w", err)
			}
			p.Model = &out
		case "model_chain":
			if err := val.Decode(&p.ModelChain); err != nil {
				return fmt.Errorf("model_chain: %w", err)
			}
		case "tools":
			p.ToolsSet = true
			if val.Kind == yaml.ScalarNode && val.Value == "all" {
				p.ToolsAll = true
			} else if err := val.Decode(&p.Tools); err != nil {
				return fmt.Errorf("tools: %w", err)
			}
		case "tools_append":
			if err := val.Decode(&p.ToolsAppend); err != nil {
				return fmt.Errorf("tools_append: %w", err)
			}
		case "tools_except":
			if err := val.Decode(&p.ToolsExcept); err != nil {
				return fmt.Errorf("tools_except: %w", err)
			}
		case "triggers":
			if err := val.Decode(&p.Triggers); err != nil {
				return fmt.Errorf("triggers: %w", err)
			}
		case "useWhen":
			if err := val.Decode(&p.UseWhen); err != nil {
				return fmt.Errorf("useWhen: %w", err)
			}
		case "avoidWhen":
			if err := val.Decode(&p.AvoidWhen); err != nil {
				return fmt.Errorf("avoidWhen: %w", err)
			}
		case "disabled":
			var out bool
			if err := val.Decode(&out); err != nil {
				return fmt.Errorf("disabled: %w", err)
			}
			p.Disabled = &out
		default:
			if !known[key] {
				p.UnknownFields = append(p.UnknownFields, key)
			}
		}
	}
	return nil
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneModels(values []ModelCandidate) []ModelCandidate {
	if values == nil {
		return nil
	}
	cloned := make([]ModelCandidate, len(values))
	copy(cloned, values)
	return cloned
}

func cloneTriggers(values []TriggerDefinition) []TriggerDefinition {
	if values == nil {
		return nil
	}
	cloned := make([]TriggerDefinition, len(values))
	copy(cloned, values)
	return cloned
}
