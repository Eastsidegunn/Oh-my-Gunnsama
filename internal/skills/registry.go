package skills

import (
	"os"
	"path/filepath"
	"sort"
)

type Scope int

const (
	ScopeProject  Scope = 0
	ScopeOpencode Scope = 1
	ScopeUser     Scope = 2
	ScopeBuiltin  Scope = 3
)

type ScopePath struct {
	Scope Scope
	Dir   string
}

type Registry struct {
	Skills map[string]Skill
}

func (r Registry) Get(name string) (Skill, bool) {
	s, ok := r.Skills[name]
	return s, ok
}

func (r Registry) All() []Skill {
	out := make([]Skill, 0, len(r.Skills))
	for _, s := range r.Skills {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func LoadAll(scopes []ScopePath) (Registry, error) {
	reg := Registry{Skills: map[string]Skill{}}
	sort.Slice(scopes, func(i, j int) bool {
		return scopes[i].Scope < scopes[j].Scope
	})
	for _, sp := range scopes {
		entries, err := os.ReadDir(sp.Dir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return Registry{}, err
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillDir := filepath.Join(sp.Dir, entry.Name())
			skill, err := Load(skillDir)
			if err != nil {
				continue
			}
			if _, exists := reg.Skills[skill.Name]; !exists {
				reg.Skills[skill.Name] = skill
			}
		}
	}
	return reg, nil
}
