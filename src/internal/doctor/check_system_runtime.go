package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// systemRuntimeCheck looks for system-installed runtimes that win
// shell resolution against dtvem's shims because their containing
// directory appears in $PATH ahead of the shims directory. When that
// happens the user's `python` (or `node`, etc.) invocation routes to
// the system binary rather than the dtvem-managed version, often
// silently — the bug isn't that dtvem failed, it's that the shell
// never asked dtvem.
//
// This is manual: fixing it means either uninstalling the system
// runtime or reordering PATH, both of which are intent decisions the
// user needs to make rather than something doctor should automate.
type systemRuntimeCheck struct {
	// Injected for testability. listProviders defaults to the global
	// runtime registry; pathEnv and shimsDir default to the live
	// environment. Each gets a stub in tests so the check can run
	// against synthetic PATH layouts without depending on whatever
	// runtimes happen to be installed on the test host.
	listProviders func() []runtime.ShimProvider
	pathEnv       func() string
	shimsDir      func() string
}

func newSystemRuntimeCheck() *systemRuntimeCheck {
	return &systemRuntimeCheck{
		listProviders: runtime.GetAllShimProviders,
		pathEnv:       func() string { return os.Getenv("PATH") },
		shimsDir:      path.ShimsDir,
	}
}

func (systemRuntimeCheck) Name() string { return "system-runtime-precedence" }

func (c systemRuntimeCheck) Run() Finding {
	providers := c.listProviders()
	if len(providers) == 0 {
		// No registered runtime providers means we can't form a list
		// of shim names to check; treat as a passing finding rather
		// than misleadingly reporting "no conflicts found".
		return Finding{OK: true, Title: "No runtime providers registered (nothing to check)"}
	}

	shimsDir := c.shimsDir()
	pathEntries := path.SplitPath(c.pathEnv())

	// Find where the shims dir sits in PATH so we can compare other
	// directories' positions against it. A shims dir absent from PATH
	// is already surfaced by shimsInPathCheck, so we skip this whole
	// check rather than double-reporting.
	shimsPos := indexOfPathEntry(pathEntries, shimsDir)
	if shimsPos < 0 {
		return Finding{OK: true, Title: "Shims dir not on PATH (covered by another check)"}
	}

	conflicts := findRuntimeConflicts(providers, pathEntries, shimsPos)
	if len(conflicts) == 0 {
		return Finding{OK: true, Title: "No system runtimes ahead of dtvem on PATH"}
	}

	// Sort by runtime name so the report is stable across runs even
	// when the conflicts map is iterated in random order internally.
	sort.Slice(conflicts, func(i, j int) bool {
		return conflicts[i].runtimeName < conflicts[j].runtimeName
	})

	details := make([]Detail, 0, len(conflicts))
	for _, c := range conflicts {
		details = append(details, Detail{
			Key:   c.displayName,
			Value: fmt.Sprintf("%s (matches: %s)", c.dir, strings.Join(c.matches, ", ")),
		})
	}

	return Finding{
		Severity:   SeverityWarning,
		Title:      fmt.Sprintf("System runtime%s on PATH before dtvem shims", plural(len(conflicts), "", "s")),
		Details:    details,
		Resolution: systemRuntimeResolution(),
	}
}

// runtimeConflict pairs a runtime's display name with the offending
// PATH directory and the specific shim names that resolved to it.
// "matches" is plural because most runtimes ship more than one
// executable (python + python3 + pip + pip3) and a single offending
// directory typically holds several of them at once.
type runtimeConflict struct {
	runtimeName string
	displayName string
	dir         string
	matches     []string
}

// findRuntimeConflicts returns one runtimeConflict per registered
// runtime that has at least one shim name appearing in a PATH
// directory positioned before shimsPos. The matched shim names are
// deduplicated and order-preserved.
func findRuntimeConflicts(providers []runtime.ShimProvider, pathEntries []string, shimsPos int) []runtimeConflict {
	var conflicts []runtimeConflict
	for _, p := range providers {
		// For each runtime, scan PATH entries that come BEFORE the
		// shims dir. The first non-shims dir containing any of the
		// runtime's executables is the offender. Stopping at the
		// first match avoids reporting every subsequent system dir
		// for the same runtime, which would just be noise.
		var firstDir string
		var matches []string
		for i, dir := range pathEntries[:shimsPos] {
			_ = i
			trimmed := strings.TrimSpace(dir)
			if trimmed == "" || isSameDir(trimmed, pathEntries[shimsPos]) {
				continue
			}
			localMatches := matchingExecutables(trimmed, p.Shims())
			if len(localMatches) == 0 {
				continue
			}
			firstDir = trimmed
			matches = localMatches
			break
		}
		if firstDir != "" {
			conflicts = append(conflicts, runtimeConflict{
				runtimeName: p.Name(),
				displayName: p.DisplayName(),
				dir:         firstDir,
				matches:     matches,
			})
		}
	}
	return conflicts
}

// matchingExecutables returns the subset of shimNames that have a
// matching executable file in dir. Platform-specific extension rules
// mirror what the shim manager uses when populating its cache.
func matchingExecutables(dir string, shimNames []string) []string {
	var matches []string
	for _, name := range shimNames {
		if executableExistsInDir(dir, name) {
			matches = append(matches, name)
		}
	}
	return matches
}

// executableExistsInDir reports whether an executable named name is
// present in dir, using the same .exe/.cmd/.bat probing the rest of
// the codebase does so we don't disagree about what "executable
// exists" means across packages.
func executableExistsInDir(dir, name string) bool {
	if goruntime.GOOS == constants.OSWindows {
		for _, ext := range []string{constants.ExtExe, constants.ExtCmd, constants.ExtBat} {
			if info, err := os.Stat(filepath.Join(dir, name+ext)); err == nil && !info.IsDir() {
				return true
			}
		}
		return false
	}
	info, err := os.Stat(filepath.Join(dir, name))
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}

// indexOfPathEntry returns the index of needle in haystack, comparing
// case-insensitively on Windows. Returns -1 when not found.
func indexOfPathEntry(haystack []string, needle string) int {
	needleClean := filepath.Clean(needle)
	for i, h := range haystack {
		if strings.TrimSpace(h) == "" {
			continue
		}
		if isSameDir(filepath.Clean(h), needleClean) {
			return i
		}
	}
	return -1
}

// isSameDir reports whether two cleaned directory paths refer to the
// same location, applying case-insensitive comparison on Windows
// since the registry can hand us mixed casing for the same path.
func isSameDir(a, b string) bool {
	if goruntime.GOOS == constants.OSWindows {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func systemRuntimeResolution() string {
	return strings.Join([]string{
		"Options:",
		"  1. Uninstall the system runtime so dtvem's shim takes over.",
		"  2. Move the offending directory below the dtvem shims directory in PATH.",
		"  3. Accept the system runtime as your default and remove dtvem's shim for that runtime.",
	}, "\n")
}

func init() {
	Register(newSystemRuntimeCheck())
}
