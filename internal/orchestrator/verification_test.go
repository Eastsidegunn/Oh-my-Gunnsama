package orchestrator

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"oh-my-gunnsama/internal/verify"
)

func TestBuildVerificationCheck_FileExistsUsesArgvForm(t *testing.T) {
	malicious := "$(touch /tmp/PWN_should_not_exist)hello.txt"
	check := buildVerificationCheck("file_exists:"+malicious, "/tmp", 0)
	if check.Shell {
		t.Fatalf("file_exists must use Shell=false (argv form), got Shell=true")
	}
	want := []string{"test", "-f", malicious}
	if len(check.Command) != len(want) {
		t.Fatalf("Command = %v, want %v", check.Command, want)
	}
	for i, v := range want {
		if check.Command[i] != v {
			t.Fatalf("Command[%d] = %q, want %q", i, check.Command[i], v)
		}
	}
}

func TestBuildVerificationCheck_FileContentUsesPositionalShell(t *testing.T) {
	maliciousPath := "$(touch /tmp/PWN_path)hello.txt"
	maliciousExpected := "`touch /tmp/PWN_expected`"
	check := buildVerificationCheck("file_content:"+maliciousPath+":"+maliciousExpected, "/tmp", 0)
	if !check.Shell {
		t.Fatalf("file_content must use Shell=true")
	}
	if strings.Contains(check.ShellCommand, maliciousPath) {
		t.Fatalf("ShellCommand must NOT contain user path (use $1 reference instead), got: %s", check.ShellCommand)
	}
	if strings.Contains(check.ShellCommand, maliciousExpected) {
		t.Fatalf("ShellCommand must NOT contain user expected value (use $2 reference instead), got: %s", check.ShellCommand)
	}
	if len(check.Command) != 2 || check.Command[0] != maliciousPath || check.Command[1] != maliciousExpected {
		t.Fatalf("Command must carry [path, expected] as positional args, got %v", check.Command)
	}
}

func TestBuildVerificationCheck_UnknownKindFailsClosed(t *testing.T) {
	check := buildVerificationCheck("weird'; touch /tmp/PWN_unknown; #:arg", "/tmp", 0)
	if check.Shell {
		t.Fatalf("unknown kind must use Shell=false (no shell interpolation possible)")
	}
	if len(check.Command) != 1 || check.Command[0] != "false" {
		t.Fatalf("unknown kind must use [false] command, got %v", check.Command)
	}
}

func TestBuildVerificationCheck_MalformedSpecFailsClosed(t *testing.T) {
	check := buildVerificationCheck("no_colon'; touch /tmp/PWN_malformed; #", "/tmp", 0)
	if check.Shell {
		t.Fatalf("malformed spec must use Shell=false")
	}
	if len(check.Command) != 1 || check.Command[0] != "false" {
		t.Fatalf("malformed spec must use [false] command, got %v", check.Command)
	}
}

func TestBuildVerificationCheck_FileContentMalformedFailsClosed(t *testing.T) {
	check := buildVerificationCheck("file_content:hello.txt", "/tmp", 0)
	if check.Shell {
		t.Fatalf("file_content without expected value must fail closed (Shell=false)")
	}
	if len(check.Command) != 1 || check.Command[0] != "false" {
		t.Fatalf("missing-expected file_content must use [false], got %v", check.Command)
	}
}

func TestBuildVerificationCheck_BashAllowsShellByDesign(t *testing.T) {
	check := buildVerificationCheck("bash:echo hello", "/tmp", 0)
	if !check.Shell {
		t.Fatalf("bash kind must use Shell=true (user explicitly opted in)")
	}
	if check.ShellCommand != "echo hello" {
		t.Fatalf("ShellCommand = %q, want 'echo hello'", check.ShellCommand)
	}
}

func TestExecRunner_FileExistsWithShellMetacharsInPath(t *testing.T) {
	tmpdir := t.TempDir()
	weirdPath := filepath.Join(tmpdir, "$(date).txt")
	if err := os.WriteFile(weirdPath, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	check := buildVerificationCheck("file_exists:"+weirdPath, tmpdir, 0)
	result, err := verify.ExecRunner{}.Run(context.Background(), check)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("file with literal $(date) in name should be found via argv, got exit=%d stderr=%q", result.ExitCode, result.Stderr)
	}
}

func TestExecRunner_FileContentWithShellMetacharsInValue(t *testing.T) {
	tmpdir := t.TempDir()
	filePath := filepath.Join(tmpdir, "hello.txt")
	value := "$(touch /tmp/PWN_should_not_be_created)"
	if err := os.WriteFile(filePath, []byte(value), 0o644); err != nil {
		t.Fatal(err)
	}
	check := buildVerificationCheck("file_content:"+filePath+":"+value, tmpdir, 0)
	result, err := verify.ExecRunner{}.Run(context.Background(), check)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("file with literal '%s' content should match, got exit=%d stderr=%q", value, result.ExitCode, result.Stderr)
	}
	if _, err := os.Stat("/tmp/PWN_should_not_be_created"); err == nil {
		os.Remove("/tmp/PWN_should_not_be_created")
		t.Fatalf("shell injection executed: /tmp/PWN_should_not_be_created was created")
	}
}
