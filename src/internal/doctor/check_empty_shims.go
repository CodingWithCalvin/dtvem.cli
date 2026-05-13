package doctor

import (
	"fmt"
	"os"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
)

// emptyShimsCheck looks for the specific broken state where the user
// has installed runtimes (so versions/ contains at least one runtime
// directory with at least one version) but the shims/ directory is
// empty or missing. In that state, `python --version` from the shell
// will route to the system runtime — or fail outright — even though
// dtvem thinks it owns the command.
//
// The fix is to run `dtvem reshim`, which is safe and idempotent: it
// recreates shims from the installed versions on disk without touching
// PATH or anything outside ~/.dtvem.
type emptyShimsCheck struct {
	// versionsDir, shimsDir, and newManager are injected so tests can
	// drive the check against a synthetic install layout without
	// touching DTVEM_ROOT (and so the fix path can be exercised
	// without producing a real dtvem-shim binary).
	versionsDir func() string
	shimsDir    func() string
	newManager  func() (rehasher, error)
}

// rehasher is the subset of shim.Manager used by emptyShimsCheck.Fix.
// Defining it locally keeps the check's tests from having to build a
// full shim.Manager (which requires a real dtvem-shim binary on disk).
type rehasher interface {
	Rehash() (*shim.RehashResult, error)
}

func newEmptyShimsCheck() *emptyShimsCheck {
	return &emptyShimsCheck{
		versionsDir: func() string { return config.DefaultPaths().Versions },
		shimsDir:    func() string { return config.DefaultPaths().Shims },
		newManager: func() (rehasher, error) {
			m, err := shim.NewManager()
			if err != nil {
				return nil, err
			}
			return m, nil
		},
	}
}

func (emptyShimsCheck) Name() string { return "empty-shims-directory" }

func (c emptyShimsCheck) Run() Finding {
	versionsCount, err := countInstalledRuntimeVersions(c.versionsDir())
	if err != nil {
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not inspect installed runtimes",
			Details:    []Detail{{Key: "Error", Value: err.Error()}},
			Resolution: "Check that " + c.versionsDir() + " is accessible.",
		}
	}

	// No installed runtimes means an empty shims/ is expected — not a
	// problem to surface to the user.
	if versionsCount == 0 {
		return Finding{OK: true, Title: "No installed runtimes (no shims expected)"}
	}

	shimsCount, err := countShimFiles(c.shimsDir())
	if err != nil {
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not inspect shims directory",
			Details:    []Detail{{Key: "Error", Value: err.Error()}},
			Resolution: "Check that " + c.shimsDir() + " is accessible.",
		}
	}

	if shimsCount > 0 {
		return Finding{OK: true, Title: "Shims directory matches installed runtimes"}
	}

	return Finding{
		Severity: SeverityError,
		Title:    "Shims directory is empty but runtimes are installed",
		Details: []Detail{
			{Key: "Versions found", Value: fmt.Sprintf("%d", versionsCount)},
			{Key: "Shims found", Value: "0"},
			{Key: "Shims dir", Value: c.shimsDir()},
		},
		Resolution: "Run `dtvem reshim` to recreate the shim files.",
		Fix: func() error {
			m, err := c.newManager()
			if err != nil {
				return fmt.Errorf("could not create shim manager: %w", err)
			}
			if _, err := m.Rehash(); err != nil {
				return fmt.Errorf("reshim failed: %w", err)
			}
			return nil
		},
	}
}

// countInstalledRuntimeVersions returns the total number of runtime
// versions installed under versionsDir. A runtime with a directory but
// no version subdirectory contributes zero.
//
// Returns (0, nil) when versionsDir doesn't exist — that's the
// "freshly initialized but no installs yet" case, which is normal and
// the caller treats as "no problem".
func countInstalledRuntimeVersions(versionsDir string) (int, error) {
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	total := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		runtimeDir := versionsDir + string(os.PathSeparator) + e.Name()
		versionEntries, err := os.ReadDir(runtimeDir)
		if err != nil {
			continue
		}
		for _, v := range versionEntries {
			if v.IsDir() {
				total++
			}
		}
	}
	return total, nil
}

// countShimFiles returns the number of non-directory entries in
// shimsDir. We don't try to distinguish .exe from .cmd wrappers on
// Windows — any file at all is enough to conclude the directory has
// been populated.
//
// Returns (0, nil) for a missing directory: that's the most common
// shape of the "empty shims" failure we're detecting.
func countShimFiles(shimsDir string) (int, error) {
	entries, err := os.ReadDir(shimsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count, nil
}

func init() {
	Register(newEmptyShimsCheck())
}
