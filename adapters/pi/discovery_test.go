package pi

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverBinary_OverrideExists(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "pi")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	got, err := DiscoverBinary(bin)
	if err != nil {
		t.Fatalf("DiscoverBinary returned error: %v", err)
	}
	if got != bin {
		t.Fatalf("got %q, want %q", got, bin)
	}
}

func TestDiscoverBinary_OverrideMissing(t *testing.T) {
	tmp := t.TempDir()
	missing := filepath.Join(tmp, "does-not-exist")
	_, err := DiscoverBinary(missing)
	if err == nil {
		t.Fatalf("DiscoverBinary(%q) returned nil error for missing file", missing)
	}
}

func TestDiscoverBinary_OverrideIsDirectory(t *testing.T) {
	tmp := t.TempDir()
	_, err := DiscoverBinary(tmp)
	if err == nil {
		t.Fatalf("DiscoverBinary(%q) returned nil error for directory", tmp)
	}
}

func TestDiscoverBinary_PATHLookup(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "pi")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Setenv("PATH", tmp)
	got, err := DiscoverBinary("")
	if err != nil {
		t.Fatalf("DiscoverBinary returned error: %v", err)
	}
	if got != bin {
		t.Fatalf("got %q, want %q", got, bin)
	}
}

func TestDiscoverBinary_PATHEmpty(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PATH", tmp)
	_, err := DiscoverBinary("")
	if err == nil {
		t.Fatalf("DiscoverBinary(\"\") returned nil error with empty PATH")
	}
	if !errors.Is(err, ErrBinaryNotFound) {
		t.Fatalf("err = %v, want errors.Is ErrBinaryNotFound", err)
	}
}
