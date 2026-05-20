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
	t.Setenv("PI_BIN", "")
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
	t.Setenv("PI_BIN", "")
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

func TestDiscoverBinary_PIBIN_EnvVar(t *testing.T) {
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "pi-from-env")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	t.Setenv("PI_BIN", bin)
	t.Setenv("PATH", t.TempDir())
	got, err := DiscoverBinary("")
	if err != nil {
		t.Fatalf("DiscoverBinary returned error: %v", err)
	}
	if got != bin {
		t.Fatalf("got %q, want %q (PI_BIN env should be honored)", got, bin)
	}
}

func TestDiscoverBinary_OverrideBeatsPIBIN(t *testing.T) {
	tmp := t.TempDir()
	binOverride := filepath.Join(tmp, "pi-from-override")
	binEnv := filepath.Join(tmp, "pi-from-env")
	if err := os.WriteFile(binOverride, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatalf("write override fixture: %v", err)
	}
	if err := os.WriteFile(binEnv, []byte("#!/bin/sh"), 0o755); err != nil {
		t.Fatalf("write env fixture: %v", err)
	}
	t.Setenv("PI_BIN", binEnv)
	got, err := DiscoverBinary(binOverride)
	if err != nil {
		t.Fatalf("DiscoverBinary: %v", err)
	}
	if got != binOverride {
		t.Fatalf("override should beat PI_BIN: got %q, want %q", got, binOverride)
	}
}

func TestDiscoverBinary_PIBINMissing(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("PI_BIN", filepath.Join(tmp, "nonexistent-pi"))
	t.Setenv("PATH", t.TempDir())
	_, err := DiscoverBinary("")
	if err == nil {
		t.Fatalf("expected error when PI_BIN points to missing file")
	}
}
