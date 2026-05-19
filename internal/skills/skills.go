package skills

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const SkillFilename = "skill.yaml"

type Skill struct {
	Name        string
	Description string
	Triggers    []Trigger
	PromptPath  string
	PromptText  string
	MCP         *MCP
}

type Trigger struct {
	Pattern  string `yaml:"pattern"`
	Priority int    `yaml:"priority"`
}

type MCP struct {
	Server     string   `yaml:"server"`
	Tools      []string `yaml:"tools"`
	EnvInherit []string `yaml:"env_inherit"`
}

type skillFile struct {
	Name        string    `yaml:"name"`
	Description string    `yaml:"description"`
	Triggers    []Trigger `yaml:"triggers"`
	Prompt      string    `yaml:"prompt"`
	MCP         *MCP      `yaml:"mcp"`
	Agent       string    `yaml:"agent"`
	Model       string    `yaml:"model"`
}

func Load(dir string) (Skill, error) {
	data, err := os.ReadFile(filepath.Join(dir, SkillFilename))
	if err != nil {
		return Skill{}, err
	}
	var file skillFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return Skill{}, err
	}
	if file.Agent != "" || file.Model != "" {
		return Skill{}, fmt.Errorf("skill schema rejected agent/model fields in v1")
	}
	if file.Name == "" {
		return Skill{}, fmt.Errorf("skill name is required")
	}
	if file.Description == "" {
		return Skill{}, fmt.Errorf("skill description is required")
	}
	skill := Skill{
		Name:        file.Name,
		Description: file.Description,
		Triggers:    file.Triggers,
		PromptPath:  file.Prompt,
		MCP:         file.MCP,
	}
	if file.Prompt != "" {
		prompt, err := os.ReadFile(filepath.Join(dir, file.Prompt))
		if err != nil {
			return Skill{}, err
		}
		skill.PromptText = string(prompt)
	}
	return skill, nil
}
