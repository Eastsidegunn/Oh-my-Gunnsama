package observer

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Scanner struct {
	timeout time.Duration
	now     func() time.Time
}

type ScannerOptions struct {
	Timeout time.Duration
	Now     func() time.Time
}

type GitCommand struct {
	Name string
	Args []string
}

func NewScanner(options ScannerOptions) *Scanner {
	if options.Timeout == 0 {
		options.Timeout = 2 * time.Second
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	return &Scanner{timeout: options.Timeout, now: options.Now}
}

func (s *Scanner) Scan(project string) (State, error) {
	abs, err := filepath.Abs(project)
	if err != nil {
		return State{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return State{}, err
	}
	if !info.IsDir() {
		return State{}, fmt.Errorf("project must be a directory")
	}
	state := State{SchemaVersion: SchemaVersion, GeneratedAt: s.now(), ProjectPath: abs, Git: GitState{ChangedFiles: []string{}}, Warnings: []string{}}
	if _, err := exec.LookPath("git"); err != nil {
		state.Git.Available = false
		state.Warnings = append(state.Warnings, "git binary not found")
		state.Cards = degradedCards("Git unavailable")
		return state, nil
	}
	state.Git.Available = true
	inside, err := s.git(abs, "rev-parse", "--is-inside-work-tree")
	if err != nil || strings.TrimSpace(inside) != "true" {
		state.Git.InsideWorkTree = false
		state.Warnings = append(state.Warnings, "project is not a git work tree")
		state.Cards = degradedCards("Not a git repository")
		return state, nil
	}
	state.Git.InsideWorkTree = true
	if branch, err := s.gitTrim(abs, "branch", "--show-current"); err == nil {
		state.Git.Branch = branch
	} else {
		state.Warnings = append(state.Warnings, "git branch scan failed: "+err.Error())
	}
	if status, err := s.git(abs, "status", "--porcelain"); err == nil {
		state.Git.StatusPorcelain = status
		state.Git.Dirty = strings.TrimSpace(state.Git.StatusPorcelain) != ""
	} else {
		state.Warnings = append(state.Warnings, "git status scan failed: "+err.Error())
	}
	if diffStat, err := s.git(abs, "diff", "--stat"); err == nil {
		state.Git.DiffStat = diffStat
	} else {
		state.Warnings = append(state.Warnings, "git diff stat scan failed: "+err.Error())
	}
	if changed, err := s.git(abs, "diff", "--name-only"); err == nil {
		state.Git.ChangedFiles = splitLines(changed)
	} else {
		state.Warnings = append(state.Warnings, "git changed-files scan failed: "+err.Error())
	}
	state.Cards = cardsFor(state)
	state.Risks = risksFor(state)
	state.SuggestedCommands = suggestionsFor(state)
	return state, nil
}

func PlannedGitCommands(project string) ([]GitCommand, error) {
	abs, err := filepath.Abs(project)
	if err != nil {
		return nil, err
	}
	return []GitCommand{
		{Name: "git", Args: []string{"-C", abs, "rev-parse", "--is-inside-work-tree"}},
		{Name: "git", Args: []string{"-C", abs, "branch", "--show-current"}},
		{Name: "git", Args: []string{"-C", abs, "status", "--porcelain"}},
		{Name: "git", Args: []string{"-C", abs, "diff", "--stat"}},
		{Name: "git", Args: []string{"-C", abs, "diff", "--name-only"}},
	}, nil
}

func ResolveOutput(project, out string) (string, error) {
	absProject, err := filepath.Abs(project)
	if err != nil {
		return "", err
	}
	if out == "" {
		out = filepath.Join(os.TempDir(), "remote-observer", filepath.Base(absProject))
	}
	absOut, err := filepath.Abs(out)
	if err != nil {
		return "", err
	}
	realProject, err := filepath.EvalSymlinks(absProject)
	if err != nil {
		return "", err
	}
	realOut := absOut
	if evaluated, err := filepath.EvalSymlinks(absOut); err == nil {
		realOut = evaluated
	} else if !os.IsNotExist(err) {
		return "", err
	}
	lexicalRel, lexicalErr := filepath.Rel(absProject, absOut)
	if lexicalErr == nil && lexicalRel != ".." && !strings.HasPrefix(lexicalRel, ".."+string(os.PathSeparator)) && lexicalRel != "." {
		return "", errors.New("output directory must be outside project")
	}
	rel, err := filepath.Rel(realProject, realOut)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && rel != "." {
		return "", errors.New("output directory must be outside project")
	}
	if realOut == realProject {
		return "", errors.New("output directory must be outside project")
	}
	return absOut, nil
}

func (s *Scanner) gitTrim(project string, args ...string) (string, error) {
	out, err := s.git(project, args...)
	return strings.TrimSpace(out), err
}

func (s *Scanner) git(project string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	argv := append([]string{"-C", project}, args...)
	cmd := exec.CommandContext(ctx, "git", argv...)
	cmd.Env = append(os.Environ(), "GIT_PAGER=cat", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), ctx.Err()
	}
	return string(out), err
}

func splitLines(s string) []string {
	lines := []string{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func degradedCards(body string) []Card {
	return []Card{{Kind: "status", Title: "Status", Body: body, Tone: "warn"}}
}

func cardsFor(state State) []Card {
	status := "Clean"
	tone := "calm"
	if state.Git.Dirty {
		status = fmt.Sprintf("Dirty workspace: %d changed files", len(state.Git.ChangedFiles))
		tone = "warn"
	}
	return []Card{
		{Kind: "status", Title: "Status", Body: status, Tone: tone},
		{Kind: "branch", Title: "Branch", Body: emptyDefault(state.Git.Branch, "unknown"), Tone: "neutral"},
		{Kind: "files", Title: "Changed Files", Body: emptyDefault(strings.Join(state.Git.ChangedFiles, "\n"), "No changed files"), Tone: tone},
		{Kind: "diff", Title: "Diff Summary", Body: emptyDefault(state.Git.DiffStat, "No diff stat"), Tone: tone},
	}
}

func risksFor(state State) []string {
	if state.Git.Dirty {
		return []string{"workspace has uncommitted changes", "tests may need to be run"}
	}
	return []string{}
}

func suggestionsFor(state State) []SuggestedCommand {
	if state.Git.Dirty {
		return []SuggestedCommand{{Target: "dev", Text: "run the relevant tests and summarize failures"}, {Target: "observer", Text: "review current diff risk"}}
	}
	return []SuggestedCommand{{Target: "dev", Text: "continue implementation or choose next task"}}
}

func emptyDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
