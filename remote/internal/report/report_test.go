package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"oh-my-gunnsama/remote/internal/observer"
)

func TestWriterWritesStateJSONAndTabletHTML(t *testing.T) {
	out := t.TempDir()
	state := observer.State{
		SchemaVersion:     1,
		GeneratedAt:       time.Date(2026, 5, 12, 4, 0, 0, 0, time.UTC),
		ProjectPath:       "/repo",
		Git:               observer.GitState{Available: true, InsideWorkTree: true, Branch: "main", Dirty: true, ChangedFiles: []string{"<script>.go"}},
		Cards:             []observer.Card{{Kind: "status", Title: "Status", Body: "Dirty", Tone: "warn"}},
		Risks:             []string{"untested"},
		SuggestedCommands: []observer.SuggestedCommand{{Target: "dev", Text: "go test ./..."}},
	}
	if err := Write(out, state); err != nil {
		t.Fatalf("Write: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(out, "state.json"))
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	var decoded observer.State
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode state: %v", err)
	}
	if decoded.SchemaVersion != 1 || decoded.ProjectPath != "/repo" {
		t.Fatalf("decoded = %#v", decoded)
	}
	html, err := os.ReadFile(filepath.Join(out, "index.html"))
	if err != nil {
		t.Fatalf("read html: %v", err)
	}
	body := string(html)
	if !strings.Contains(body, "Remote Observer") || !strings.Contains(body, "state.json") {
		t.Fatalf("html missing shell: %s", body)
	}
	if strings.Contains(body, "<script>.go") {
		t.Fatalf("html should not inline unsafe filename unescaped: %s", body)
	}
}
