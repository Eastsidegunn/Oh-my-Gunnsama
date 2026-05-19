package reportserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestServerServesReportFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>Remote Observer</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(dir)
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "Remote Observer") {
		t.Fatalf("index response = %d %q", resp.Code, resp.Body.String())
	}
	resp = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/state.json", nil)
	srv.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), "ok") {
		t.Fatalf("state response = %d %q", resp.Code, resp.Body.String())
	}
}

func TestServerRefreshesReportViaInjectedRefresh(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	srv := NewWithRefresh(dir, func() error {
		calls++
		return os.WriteFile(filepath.Join(dir, "state.json"), []byte(`{"calls":1}`), 0o644)
	})
	resp := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/refresh", nil)
	srv.Handler().ServeHTTP(resp, req)
	if resp.Code != http.StatusOK || calls != 1 {
		t.Fatalf("refresh code=%d calls=%d body=%q", resp.Code, calls, resp.Body.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "state.json")); err != nil {
		t.Fatalf("state not refreshed: %v", err)
	}
}
