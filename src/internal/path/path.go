// Package path provides utilities for PATH environment variable manipulation
package path

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// dtvemMarkerComment is the literal comment line dtvem writes above
// every PATH export it adds to a Unix shell config. The stale-shims
// fix removes a PATH export only when this exact marker is on the
// line above, so we never touch lines the user wrote themselves.
const dtvemMarkerComment = "# Added by dtvem"

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
	_, stale := PartitionStaleShimsEntries(pathEntries, currentShimsDir)
	return stale
}

// PartitionStaleShimsEntries splits pathEntries into the entries to keep
// and the stale dtvem-shims entries to remove. Order is preserved in
// both slices, and the strings are the original (un-cleaned) entries so
// callers can match them against registry-stored values verbatim.
//
// An empty string entry is treated as "drop silently" — it lands in
// neither slice, which matches what callers want when rewriting PATH
// (we don't want to preserve stray empty entries that arose from
// trailing separators).
//
// When currentShimsDir is empty the entire input is returned as kept,
// since without a "current" reference we can't decide which dtvem-shims
// entries are stale and which are correct.
func PartitionStaleShimsEntries(pathEntries []string, currentShimsDir string) (kept, stale []string) {
	if currentShimsDir == "" {
		kept = make([]string, 0, len(pathEntries))
		for _, entry := range pathEntries {
			if strings.TrimSpace(entry) == "" {
				continue
			}
			kept = append(kept, entry)
		}
		return kept, nil
	}
	currentClean := filepath.Clean(currentShimsDir)

	for _, entry := range pathEntries {
		trimmed := strings.TrimSpace(entry)
		if trimmed == "" {
			continue
		}
		if !IsDtvemShimsPath(trimmed) {
			kept = append(kept, entry)
			continue
		}
		entryClean := filepath.Clean(trimmed)
		matches := entryClean == currentClean
		if runtime.GOOS == constants.OSWindows {
			matches = strings.EqualFold(entryClean, currentClean)
		}
		if matches {
			kept = append(kept, entry)
			continue
		}
		stale = append(stale, entry)
	}
	return kept, stale
}

// RemoveStaleShimsFromShellConfig removes "marker + export" blocks
// from a single shell config file when the export references a dtvem
// shims directory that doesn't match currentShimsDir. The file is
// rewritten in place; the returned slice lists the stale paths whose
// blocks were dropped, in source order.
//
// Only pairs of lines where the first line is exactly the dtvem
// marker comment are touched. That keeps the fix safe against user-
// edited configs: a PATH export the user wrote themselves won't have
// the marker above it and will be left alone even if it happens to
// reference an old dtvem path.
//
// If the file doesn't exist or contains nothing to remove, the file
// is left alone and the returned slice is empty. The function is
// safe to call on Windows but won't normally find anything there
// because the Windows install path stores PATH in the registry, not
// in a config file — the doctor command's Windows fix routes through
// RemoveStaleShimsFromUserPath instead.
func RemoveStaleShimsFromShellConfig(configFile, currentShimsDir string) ([]string, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", configFile, err)
	}

	newContent, removed := removeDtvemMarkerBlocks(string(data), currentShimsDir)
	if len(removed) == 0 {
		return nil, nil
	}

	if err := os.WriteFile(configFile, []byte(newContent), 0644); err != nil {
		return nil, fmt.Errorf("write %s: %w", configFile, err)
	}
	return removed, nil
}

// removeDtvemMarkerBlocks scans configContent line by line, dropping
// each pair of (marker comment, following export line) whose export
// line references a dtvem shims path that doesn't match
// currentShimsDir. Returns the rewritten content and the stale paths
// that were dropped, in source order.
//
// Behavior intentionally errs on the side of keeping content:
//   - A marker comment that isn't followed by anything is preserved.
//   - A marker comment followed by an export that doesn't look like a
//     dtvem shims path is preserved (some user wrote something
//     unrelated under our marker).
//   - A marker comment followed by an export pointing at the current
//     shims dir is preserved (it's not stale).
func removeDtvemMarkerBlocks(configContent, currentShimsDir string) (string, []string) {
	lines := strings.Split(configContent, "\n")
	out := make([]string, 0, len(lines))
	var removed []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.TrimSpace(line) != dtvemMarkerComment || i+1 >= len(lines) {
			out = append(out, line)
			continue
		}

		next := lines[i+1]
		stale := extractStaleShimsPath(next, currentShimsDir)
		if stale == "" {
			out = append(out, line)
			continue
		}

		// Drop both the marker and the export, recording what we
		// removed so the caller can report it to the user.
		removed = append(removed, stale)
		i++
	}

	return strings.Join(out, "\n"), removed
}

// extractStaleShimsPath returns the dtvem shims path referenced in a
// PATH-export line if and only if that path is non-empty, looks like a
// dtvem shims dir, and is not currentShimsDir. Returns "" when the
// line isn't a stale-referencing export.
//
// The matcher is permissive across bash/zsh ("export PATH=...") and
// fish ("set -gx PATH ...") because the dtvem shims path appears
// inside double quotes in both cases. We pull every quoted substring,
// split each on the path separator to drop the trailing $PATH the
// bash export concatenates, and check the first segment against
// IsDtvemShimsPath.
func extractStaleShimsPath(line, currentShimsDir string) string {
	currentClean := filepath.Clean(currentShimsDir)

	for _, quoted := range quotedSubstrings(line) {
		// Bash-style "<dir>:$PATH" — keep the part before the first
		// colon. Fish-style "<dir>" — the whole quoted string is the
		// path, so the no-colon case is a no-op.
		candidate := quoted
		if idx := strings.Index(candidate, ":"); idx >= 0 {
			candidate = candidate[:idx]
		}
		candidate = strings.TrimSpace(candidate)
		if !IsDtvemShimsPath(candidate) {
			continue
		}
		if filepath.Clean(candidate) == currentClean {
			// References the current shims dir — that's the active
			// export we want to leave in place.
			continue
		}
		return candidate
	}
	return ""
}

// quotedSubstrings returns every substring of s that is enclosed in
// double quotes, in source order. We deliberately only match "..."
// (not '...' or backticks) because every dtvem-written PATH export
// uses double quotes; accepting other styles would broaden the match
// surface for user-written lines we don't want to touch.
func quotedSubstrings(s string) []string {
	var out []string
	i := 0
	for {
		start := strings.IndexByte(s[i:], '"')
		if start < 0 {
			return out
		}
		start += i + 1
		end := strings.IndexByte(s[start:], '"')
		if end < 0 {
			return out
		}
		out = append(out, s[start:start+end])
		i = start + end + 1
	}
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
