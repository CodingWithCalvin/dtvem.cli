package doctor

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// xdgSplitCheck detects the case where dtvem has data installed at
// more than one of its candidate root locations. The classic split
// is: a user originally installed dtvem when XDG_DATA_HOME was unset
// (so versions landed under ~/.dtvem), then later set XDG_DATA_HOME
// and reinstalled — the active install now lives elsewhere, but
// shims/configs/versions in ~/.dtvem still exist and clutter the
// system or, worse, get used by an outdated dtvem binary.
//
// We don't attempt to auto-migrate: that's an intent decision (which
// data to keep, whether to consolidate to XDG or back to ~/.dtvem)
// and it's destructive enough to deserve an explicit user choice.
type xdgSplitCheck struct {
	// Injected so tests can drive XDG/home/DTVEM_ROOT permutations
	// deterministically without mutating real env vars and
	// directories.
	activeRoot     func() string
	candidateRoots func() []string
	hasData        func(root string) bool
}

func newXDGSplitCheck() *xdgSplitCheck {
	return &xdgSplitCheck{
		activeRoot:     func() string { return config.DefaultPaths().Root },
		candidateRoots: candidateDtvemRoots,
		hasData:        rootHasData,
	}
}

func (xdgSplitCheck) Name() string { return "xdg-split-state" }

func (c xdgSplitCheck) Run() Finding {
	active := c.activeRoot()
	cleanActive := filepath.Clean(active)

	var orphans []string
	for _, candidate := range c.candidateRoots() {
		if candidate == "" {
			continue
		}
		cleanCandidate := filepath.Clean(candidate)
		if pathsEqual(cleanCandidate, cleanActive) {
			continue
		}
		if c.hasData(candidate) {
			orphans = append(orphans, candidate)
		}
	}

	if len(orphans) == 0 {
		return Finding{OK: true, Title: "Only one dtvem install location holds data"}
	}

	details := []Detail{{Key: "Active install", Value: active}}
	for _, o := range orphans {
		details = append(details, Detail{Key: "Stale install", Value: o})
	}

	return Finding{
		Severity:   SeverityWarning,
		Title:      "dtvem data exists at more than one install location",
		Details:    details,
		Resolution: xdgSplitResolution(),
	}
}

// candidateDtvemRoots returns every directory where a dtvem install
// could plausibly have data: the DTVEM_ROOT override, the XDG-derived
// path, and the home-relative ~/.dtvem path. Order is intentional —
// active root first — so the caller can dedupe trivially against
// activeRoot.
func candidateDtvemRoots() []string {
	var out []string

	if v := os.Getenv("DTVEM_ROOT"); v != "" {
		out = append(out, v)
	}

	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		out = append(out, filepath.Join(xdg, "dtvem"))
	}

	if home, err := os.UserHomeDir(); err == nil && home != "" {
		// ~/.dtvem (macOS/Windows default, also a common pre-XDG
		// Linux layout).
		out = append(out, filepath.Join(home, ".dtvem"))
		// ~/.local/share/dtvem (Linux XDG default with XDG_DATA_HOME
		// unset, and a common alternate spot if the user switches
		// XDG_DATA_HOME between sessions).
		if goruntime.GOOS == constants.OSLinux {
			out = append(out, filepath.Join(home, ".local", "share", "dtvem"))
		}
	}

	return dedupePaths(out)
}

// rootHasData reports whether root looks like a populated dtvem
// install. We require at least one runtime version directory to
// avoid flagging brand-new installs that have created the directory
// tree but not yet downloaded anything — those aren't "data" the
// user could reasonably reuse or care about migrating.
func rootHasData(root string) bool {
	versionsDir := filepath.Join(root, "versions")
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		return false
	}
	for _, runtime := range entries {
		if !runtime.IsDir() {
			continue
		}
		versions, err := os.ReadDir(filepath.Join(versionsDir, runtime.Name()))
		if err != nil {
			continue
		}
		for _, v := range versions {
			if v.IsDir() {
				return true
			}
		}
	}
	return false
}

// dedupePaths returns paths with duplicates removed, comparing
// case-insensitively on Windows. Order of first occurrence is
// preserved.
func dedupePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		key := filepath.Clean(p)
		if goruntime.GOOS == constants.OSWindows {
			key = strings.ToLower(key)
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

// pathsEqual reports whether two cleaned paths refer to the same
// directory, with case-insensitive comparison on Windows.
func pathsEqual(a, b string) bool {
	if goruntime.GOOS == constants.OSWindows {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func xdgSplitResolution() string {
	return strings.Join([]string{
		"Pick one install location and consolidate:",
		"  - If the 'Active install' is the one you want to keep, delete the 'Stale install' tree(s)",
		"    after moving any versions you still need across with `dtvem install`.",
		"  - If a 'Stale install' is actually the one you want to keep, unset XDG_DATA_HOME (or set",
		"    DTVEM_ROOT to point at it) and re-run `dtvem init` so the active install matches.",
		"Always restart your terminal after consolidating so PATH picks up the change.",
	}, "\n")
}

func init() {
	Register(newXDGSplitCheck())
}
