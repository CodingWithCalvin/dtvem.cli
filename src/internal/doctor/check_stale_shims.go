package doctor

import (
	"fmt"
	"os"
	goruntime "runtime"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
)

// staleShimsCheck looks for entries in $PATH that match the dtvem
// shims-directory pattern but don't equal the current resolved shims
// directory. These are usually left over from a pre-XDG install or
// from switching install types (system vs. user PATH on Windows).
//
// The check is fixable: on Windows we surgically remove stale entries
// from the User PATH registry value (no admin needed); on Unix we
// rewrite the user's shell config to drop "# Added by dtvem"-marked
// blocks that point at stale paths. System PATH stale entries on
// Windows still require admin and remain a manual step; we surface
// that explicitly in the report when applicable.
type staleShimsCheck struct {
	// Injected for testability. Each closure defaults to a real
	// implementation; tests override individually so the check can be
	// exercised across success, no-op, and platform-specific paths
	// without touching the actual registry or shell config.
	pathEnv         func() string
	currentShimsDir func() string
	userHomeDir     func() (string, error)
	removeFromPath  func(currentShimsDir string) ([]string, error)
	removeFromFile  func(configFile, currentShimsDir string) ([]string, error)
	detectShell     func() string
	shellConfigFile func(shell string) string
}

func newStaleShimsCheck() *staleShimsCheck {
	return &staleShimsCheck{
		pathEnv:         func() string { return os.Getenv("PATH") },
		currentShimsDir: path.ShimsDir,
		userHomeDir:     os.UserHomeDir,
		removeFromPath:  removeStaleShimsFromUserPath,
		removeFromFile:  path.RemoveStaleShimsFromShellConfig,
		detectShell:     path.DetectShell,
		shellConfigFile: path.GetShellConfigFile,
	}
}

func (staleShimsCheck) Name() string { return "stale-shims-path" }

func (c *staleShimsCheck) Run() Finding {
	currentShims := c.currentShimsDir()
	pathEntries := path.SplitPath(c.pathEnv())
	stale := path.FindStaleShimsEntries(pathEntries, currentShims)

	if len(stale) == 0 {
		return Finding{OK: true, Title: "No stale dtvem shims entries in PATH"}
	}

	details := []Detail{{Key: "Expected", Value: currentShims}}
	for _, s := range stale {
		details = append(details, Detail{Key: "Found", Value: s})
	}

	resolution, fix := c.fixOrInstructions(stale, currentShims)
	return Finding{
		Severity:   SeverityError,
		Title:      fmt.Sprintf("Stale dtvem shims entr%s in PATH", plural(len(stale), "y", "ies")),
		Details:    details,
		Resolution: resolution,
		Fix:        fix,
	}
}

// fixOrInstructions returns the resolution text and (optionally) a Fix
// closure for the current platform. On both platforms we always return
// instructions, but only return a Fix when we have a safe in-place
// remediation: User PATH registry rewrite on Windows, shell-config
// marker-block removal on Unix.
func (c *staleShimsCheck) fixOrInstructions(stale []string, currentShims string) (string, func() error) {
	if goruntime.GOOS == constants.OSWindows {
		return c.windowsFix(stale, currentShims)
	}
	return c.unixFix(currentShims)
}

func (c *staleShimsCheck) windowsFix(stale []string, currentShims string) (string, func() error) {
	resolution := strings.Join([]string{
		"Remove the stale entries from your User PATH (no admin required).",
		"If the entries are in System PATH, you'll need to run this from an",
		"elevated terminal — doctor will only touch User PATH automatically.",
		"Restart your terminal after the fix for the change to take effect.",
	}, "\n")

	fix := func() error {
		removed, err := c.removeFromPath(currentShims)
		if err != nil {
			return err
		}
		if len(removed) == 0 {
			// User PATH didn't actually contain the stale entries, so
			// they must be in System PATH (which we can't write to
			// without admin). Surface that distinctly rather than
			// silently succeeding — the user would otherwise expect
			// the warning to disappear and be confused when it doesn't.
			return fmt.Errorf("no stale entries found in User PATH; %d entr%s likely in System PATH and require an elevated terminal",
				len(stale), plural(len(stale), "y is", "ies are"))
		}
		return nil
	}
	return resolution, fix
}

func (c *staleShimsCheck) unixFix(currentShims string) (string, func() error) {
	shell := c.detectShell()
	configFile := c.shellConfigFile(shell)
	if configFile == "" {
		return strings.Join([]string{
			"Could not auto-detect your shell config file; edit your shell rc",
			"manually and remove the export line(s) that reference the stale",
			"'Found' paths above. Restart your terminal afterward.",
		}, "\n"), nil
	}

	resolution := strings.Join([]string{
		"Remove the '# Added by dtvem' marker block(s) referencing stale paths from",
		"  " + configFile,
		"After the fix, restart your terminal or 'source' the file.",
	}, "\n")

	fix := func() error {
		removed, err := c.removeFromFile(configFile, currentShims)
		if err != nil {
			return err
		}
		if len(removed) == 0 {
			// The export must live somewhere we don't auto-detect (a
			// custom config file, /etc/profile, etc.). Tell the user
			// what we looked at so they know where to keep searching.
			return fmt.Errorf("no '# Added by dtvem' marker blocks found in %s; the stale export(s) may live in another config file", configFile)
		}
		return nil
	}
	return resolution, fix
}

// removeStaleShimsFromUserPath is the default Windows-side fix used by
// the staleShimsCheck. The actual registry write lives in
// internal/path/path_windows.go; on Unix builds the impl returns a
// sentinel error since the cross-platform code path in staleShimsCheck
// doesn't reach this branch.
//
// This indirection lets the check stay compile-time portable while
// keeping the real implementation behind build tags where it belongs.
func removeStaleShimsFromUserPath(currentShimsDir string) ([]string, error) {
	return removeStaleShimsFromUserPathImpl(currentShimsDir)
}

func init() {
	Register(newStaleShimsCheck())
}
