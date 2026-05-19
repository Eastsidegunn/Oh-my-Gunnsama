package observer

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScanNonGitDirectoryReturnsDegradedReport(t *testing.T) {
	dir := t.TempDir()
	scanner := NewScanner(ScannerOptions{Timeout: 2 * time.Second})
	state, err := scanner.Scan(dir)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if state.SchemaVersion != 1 || state.ProjectPath != dir {
		t.Fatalf("state identity mismatch: %#v", state)
	}
	if !state.Git.Available || state.Git.InsideWorkTree {
		t.Fatalf("git degraded shape mismatch: %#v", state.Git)
	}
	if len(state.Warnings) == 0 || len(state.Cards) == 0 {
		t.Fatalf("expected warnings and cards: %#v", state)
	}
}

func TestScanGitRepoReportsBranchDirtyFilesAndDiffStat(t *testing.T) {
	dir := initGitRepo(t)
	writeFile(t, filepath.Join(dir, "file.txt"), "hello\nchanged\n")
	scanner := NewScanner(ScannerOptions{Timeout: 2 * time.Second})
	state, err := scanner.Scan(dir)
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}
	if !state.Git.InsideWorkTree || !state.Git.Dirty {
		t.Fatalf("git state = %#v", state.Git)
	}
	if state.Git.Branch == "" {
		t.Fatalf("branch should be reported: %#v", state.Git)
	}
	if !contains(state.Git.ChangedFiles, "file.txt") {
		t.Fatalf("changed files = %#v", state.Git.ChangedFiles)
	}
	if !strings.Contains(state.Git.DiffStat, "file.txt") {
		t.Fatalf("diff stat = %q", state.Git.DiffStat)
	}
	if len(state.SuggestedCommands) == 0 || len(state.Risks) == 0 {
		t.Fatalf("expected suggestions/risks: %#v", state)
	}
}

func TestAllowedGitCommandsUseArgvNoShellAndAbsProject(t *testing.T) {
	project := filepath.Join(t.TempDir(), "repo with spaces")
	commands, err := PlannedGitCommands(project)
	if err != nil {
		t.Fatalf("PlannedGitCommands: %v", err)
	}
	abs, _ := filepath.Abs(project)
	if len(commands) != 5 {
		t.Fatalf("commands = %#v", commands)
	}
	for _, cmd := range commands {
		if cmd.Name != "git" {
			t.Fatalf("command must be git argv, got %#v", cmd)
		}
		joined := strings.Join(cmd.Args, " ")
		if strings.Contains(joined, "sh -c") || strings.Contains(joined, ";") {
			t.Fatalf("command uses shell-like args: %#v", cmd)
		}
		if len(cmd.Args) < 3 || cmd.Args[0] != "-C" || cmd.Args[1] != abs {
			t.Fatalf("command must use git -C absProject: %#v", cmd)
		}
	}
}

func TestValidateOutputOutsideProject(t *testing.T) {
	project := t.TempDir()
	inside := filepath.Join(project, "report")
	if _, err := ResolveOutput(project, inside); err == nil {
		t.Fatalf("expected output inside project to fail")
	}
	outside := filepath.Join(t.TempDir(), "report")
	resolved, err := ResolveOutput(project, outside)
	if err != nil {
		t.Fatalf("outside output should pass: %v", err)
	}
	if resolved != outside {
		t.Fatalf("resolved = %q want %q", resolved, outside)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test")
	writeFile(t, filepath.Join(dir, "file.txt"), "hello\n")
	runGit(t, dir, "add", "file.txt")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestResolveOutputRejectsSymlinkPointingInsideProject(t *testing.T) {
	project := t.TempDir()
	inside := filepath.Join(project, "actual-report")
	if err := os.MkdirAll(inside, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideLink := filepath.Join(t.TempDir(), "report-link")
	if err := os.Symlink(inside, outsideLink); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveOutput(project, outsideLink); err == nil {
		t.Fatalf("expected symlink to project-internal output to fail")
	}
}

func TestDefaultOutputOutsideProject(t *testing.T) {
	project := t.TempDir()
	out, err := ResolveOutput(project, "")
	if err != nil {
		t.Fatalf("ResolveOutput default: %v", err)
	}
	rel, _ := filepath.Rel(project, out)
	if rel == "." || (!strings.HasPrefix(rel, "..") && rel != "") {
		t.Fatalf("default output should be outside project: %s", out)
	}
}

func TestPlannedGitCommandsHandleLeadingDashPathAsArgv(t *testing.T) {
	base := t.TempDir()
	project := filepath.Join(base, "-dash repo")
	commands, err := PlannedGitCommands(project)
	if err != nil {
		t.Fatalf("PlannedGitCommands: %v", err)
	}
	abs, _ := filepath.Abs(project)
	for _, cmd := range commands {
		if cmd.Args[0] != "-C" || cmd.Args[1] != abs {
			t.Fatalf("project path must be argv after -C: %#v", cmd)
		}
	}
}

func TestGitTimeoutProducesWarningNotFailure(t *testing.T) {
	dir := initGitRepo(t)
	scanner := NewScanner(ScannerOptions{Timeout: time.Nanosecond})
	state, err := scanner.Scan(dir)
	if err != nil {
		t.Fatalf("timeout should degrade, not fail: %v", err)
	}
	if len(state.Warnings) == 0 {
		t.Fatalf("expected timeout/degraded warning: %#v", state)
	}
}
