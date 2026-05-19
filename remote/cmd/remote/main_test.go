package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestObserveDefaultOnceWritesReportAndExits(t *testing.T) {
	project := t.TempDir()
	out := filepath.Join(t.TempDir(), "report")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"observe", "--project", project, "--out", out}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(out, "state.json")); err != nil {
		t.Fatalf("state missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Fatalf("html missing: %v", err)
	}
	if strings.Contains(stdout.String(), "token") {
		t.Fatalf("observe should not print token/pairing: %s", stdout.String())
	}
}

func TestObserveRejectsMissingProjectAndInsideOutput(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"observe", "--project", filepath.Join(t.TempDir(), "missing"), "--out", t.TempDir()}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected missing project failure")
	}
	project := t.TempDir()
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"observe", "--project", project, "--out", filepath.Join(project, "report")}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected inside output failure")
	}
}

func TestObserveRejectsConflictingModesAndLowRefresh(t *testing.T) {
	project := t.TempDir()
	out := filepath.Join(t.TempDir(), "report")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"observe", "--project", project, "--out", out, "--once", "--serve"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected conflicting mode failure")
	}
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"observe", "--project", project, "--out", out, "--serve", "--refresh", "500ms"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected low refresh failure")
	}
}

func TestObserveOncePrintsJSONWhenRequested(t *testing.T) {
	project := t.TempDir()
	out := filepath.Join(t.TempDir(), "report")
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"observe", "--project", project, "--out", out, "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code=%d stderr=%s", code, stderr.String())
	}
	var info struct {
		Out     string `json:"out"`
		Project string `json:"project"`
		Served  bool   `json:"served"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &info); err != nil {
		t.Fatalf("decode json %q: %v", stdout.String(), err)
	}
	if info.Out != out || info.Project != project || info.Served {
		t.Fatalf("info = %#v", info)
	}
}

func TestControlRoomCommandRemoved(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"control-room", "start"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("control-room command should be removed")
	}
}
