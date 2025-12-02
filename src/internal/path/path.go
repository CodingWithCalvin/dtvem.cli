// Package path provides utilities for PATH environment variable manipulation
package path

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// IsInPath checks if a directory is in the system PATH
func IsInPath(dir string) bool {
	pathEnv := os.Getenv("PATH")

	// Get the path separator for this OS
	separator := ":"
	if runtime.GOOS == "windows" {
		separator = ";"
	}

	// Split PATH into individual directories
	paths := strings.Split(pathEnv, separator)

	// Normalize the directory path for comparison
	dir = filepath.Clean(dir)

	for _, p := range paths {
		p = filepath.Clean(p)
		if p == dir {
			return true
		}
	}

	return false
}

// ShimsDir returns the path to the shims directory
// This is a helper to avoid circular dependencies with config package
func ShimsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".dtvem", "shims")
}
