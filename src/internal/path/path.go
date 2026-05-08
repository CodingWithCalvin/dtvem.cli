// Package path provides utilities for PATH environment variable manipulation
package path

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// IsInPath checks if a directory is in the system PATH
func IsInPath(dir string) bool {
	pathEnv := os.Getenv("PATH")

	// Get the path separator for this OS
	separator := ":"
	if runtime.GOOS == constants.OSWindows {
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

// IsDtvemShimsPath reports whether path looks like a dtvem shims directory.
// It matches the standard installation patterns:
//   - <anything>/dtvem/shims  (e.g., ~/.local/share/dtvem/shims under XDG_DATA_HOME)
//   - <anything>/.dtvem/shims (the default Windows/macOS layout, leading dot)
//
// Comparison is case-insensitive on Windows. Custom DTVEM_ROOT layouts whose
// final two components don't match these patterns are not detected.
func IsDtvemShimsPath(path string) bool {
	if path == "" {
		return false
	}

	cleaned := filepath.Clean(path)
	leaf := filepath.Base(cleaned)
	parent := filepath.Base(filepath.Dir(cleaned))

	leafEq := func(a, b string) bool { return a == b }
	if runtime.GOOS == constants.OSWindows {
		leafEq = strings.EqualFold
	}

	if !leafEq(leaf, "shims") {
		return false
	}
	return leafEq(parent, "dtvem") || leafEq(parent, ".dtvem")
}

// FindStaleShimsEntries scans pathEntries for entries that look like dtvem
// shims directories but do not match currentShimsDir. The returned slice
// preserves the order of appearance in pathEntries and has the original
// (un-cleaned) entry strings, so callers can match them against registry
// or config-file content.
//
// Comparison against currentShimsDir is case-insensitive on Windows.
func FindStaleShimsEntries(pathEntries []string, currentShimsDir string) []string {
	if currentShimsDir == "" {
		return nil
	}
	currentClean := filepath.Clean(currentShimsDir)

	var stale []string
	for _, entry := range pathEntries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if !IsDtvemShimsPath(trimmed) {
			continue
		}
		entryClean := filepath.Clean(trimmed)
		if runtime.GOOS == constants.OSWindows {
			if strings.EqualFold(entryClean, currentClean) {
				continue
			}
		} else {
			if entryClean == currentClean {
				continue
			}
		}
		stale = append(stale, entry)
	}
	return stale
}

// SplitPath splits the PATH environment variable using the OS-appropriate separator.
func SplitPath(pathEnv string) []string {
	separator := ":"
	if runtime.GOOS == constants.OSWindows {
		separator = ";"
	}
	return strings.Split(pathEnv, separator)
}

// ShimsDir returns the path to the shims directory
// This replicates the root directory logic from config package to avoid circular dependencies.
// Must stay in sync with config.getRootDir().
func ShimsDir() string {
	// Check for DTVEM_ROOT environment variable first (overrides all)
	if root := os.Getenv("DTVEM_ROOT"); root != "" {
		return filepath.Join(root, "shims")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// On Linux, respect XDG Base Directory specification
	if runtime.GOOS == "linux" {
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "dtvem", "shims")
		}
		// XDG default: ~/.local/share
		return filepath.Join(home, ".local", "share", "dtvem", "shims")
	}

	// On macOS and Windows, use XDG_DATA_HOME if explicitly set (opt-in)
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "dtvem", "shims")
	}

	// Default for macOS and Windows: ~/.dtvem
	return filepath.Join(home, ".dtvem", "shims")
}

// LookPathExcludingShims searches for an executable in PATH, excluding dtvem's shims directory.
// This prevents detecting our own shims as "system" installations during migration detection.
// Returns the full path to the executable, or empty string if not found.
func LookPathExcludingShims(execName string) string {
	// Get the shims directory to exclude it from search
	shimsDir := ShimsDir()

	// Get PATH environment variable
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return ""
	}

	// Split PATH into directories
	pathDirs := filepath.SplitList(pathEnv)

	// Search each directory
	for _, dir := range pathDirs {
		// Skip the dtvem shims directory (case-insensitive on Windows)
		if strings.EqualFold(dir, shimsDir) {
			continue
		}

		// Try to find the executable in this directory
		candidatePath := findExecutableInDir(dir, execName)
		if candidatePath != "" {
			return candidatePath
		}
	}

	return ""
}

// findExecutableInDir looks for an executable with the given name in a directory.
// On Windows, it tries .exe, .cmd, .bat extensions.
// On Unix, it checks if the file exists and has execute permission.
func findExecutableInDir(dir, execName string) string {
	if runtime.GOOS == constants.OSWindows {
		// Windows: try .exe, .cmd, .bat extensions
		for _, ext := range []string{constants.ExtExe, constants.ExtCmd, constants.ExtBat} {
			candidate := filepath.Join(dir, execName+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	} else {
		// Unix: check if file exists and is executable
		candidate := filepath.Join(dir, execName)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			// Check if executable (has execute permission)
			if info.Mode()&0111 != 0 {
				return candidate
			}
		}
	}
	return ""
}
