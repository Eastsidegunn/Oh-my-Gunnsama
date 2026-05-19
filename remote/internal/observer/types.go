package observer

import "time"

const SchemaVersion = 1

type State struct {
	SchemaVersion     int                `json:"schema_version"`
	GeneratedAt       time.Time          `json:"generated_at"`
	ProjectPath       string             `json:"project_path"`
	Git               GitState           `json:"git"`
	Cards             []Card             `json:"cards"`
	Risks             []string           `json:"risks"`
	SuggestedCommands []SuggestedCommand `json:"suggested_commands"`
	Warnings          []string           `json:"warnings"`
}

type GitState struct {
	Available       bool     `json:"available"`
	InsideWorkTree  bool     `json:"inside_work_tree"`
	Branch          string   `json:"branch,omitempty"`
	Dirty           bool     `json:"dirty"`
	StatusPorcelain string   `json:"status_porcelain,omitempty"`
	DiffStat        string   `json:"diff_stat,omitempty"`
	ChangedFiles    []string `json:"changed_files"`
}

type Card struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Tone  string `json:"tone"`
}

type SuggestedCommand struct {
	Target string `json:"target"`
	Text   string `json:"text"`
}
