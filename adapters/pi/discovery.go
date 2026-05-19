// Package pi implements the OmG Worker adapter for the pi-coding-agent subprocess.
package pi

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

// ErrBinaryNotFound is returned when the pi binary cannot be located.
var ErrBinaryNotFound = errors.New("pi binary not found")

// DiscoverBinary resolves the path to the pi executable.
//
// Resolution order:
//  1. If override is non-empty, verify it exists and is a regular file; return it.
//  2. Look up "pi" in $PATH via os/exec.LookPath; return the resolved path.
//  3. Return ErrBinaryNotFound wrapped with context.
//
// Environment variables other than $PATH (which LookPath uses) are not consulted.
func DiscoverBinary(override string) (string, error) {
	if override != "" {
		info, err := os.Stat(override)
		if err != nil {
			return "", fmt.Errorf("pi binary override %q: %w", override, err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("pi binary override %q: is a directory", override)
		}
		return override, nil
	}
	path, err := exec.LookPath("pi")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrBinaryNotFound, err)
	}
	return path, nil
}
