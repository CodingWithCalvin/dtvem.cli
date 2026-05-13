package doctor

import (
	"fmt"
	"os"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
)

// shimsInPathCheck verifies that the currently resolved shims directory
// is on $PATH. If it isn't, dtvem-managed runtimes won't be reachable
// from the shell — the shims exist but nothing routes the user's
// `python` invocation to them.
//
// The remediation is `dtvem init`, which is the same command users run
// at install time. We don't auto-invoke it here because init() writes
// to the user's shell config (Unix) or registry (Windows) and the user
// should opt in to that explicitly, separately from running `doctor`.
type shimsInPathCheck struct{}

func newShimsInPathCheck() *shimsInPathCheck { return &shimsInPathCheck{} }

func (shimsInPathCheck) Name() string { return "shims-in-path" }

func (c shimsInPathCheck) Run() Finding {
	shims := path.ShimsDir()
	if shims == "" {
		// path.ShimsDir() only returns "" when UserHomeDir fails and
		// DTVEM_ROOT is unset — extremely rare, but the check would
		// produce a misleading result if we treated empty as a hit, so
		// we surface it as a distinct problem instead.
		return Finding{
			Severity:   SeverityError,
			Title:      "Could not resolve dtvem shims directory",
			Resolution: "Set DTVEM_ROOT to your dtvem install location, or ensure your home directory is accessible.",
		}
	}

	if path.IsInPath(shims) {
		return Finding{OK: true, Title: "dtvem shims directory is on PATH"}
	}

	return Finding{
		Severity: SeverityError,
		Title:    "dtvem shims directory is not on PATH",
		Details: []Detail{
			{Key: "Expected", Value: shims},
			{Key: "Status", Value: pathEnvSummary()},
		},
		Resolution: "Run `dtvem init` to add the shims directory to your PATH.\nIf you've already run init, restart your terminal so the PATH change takes effect.",
	}
}

// pathEnvSummary returns a short human-readable description of $PATH so
// the user can see whether PATH is genuinely missing the entry vs. just
// hasn't been reloaded in the current shell.
func pathEnvSummary() string {
	pathEnv := os.Getenv("PATH")
	if pathEnv == "" {
		return "$PATH is empty"
	}
	entries := path.SplitPath(pathEnv)
	// Drop empty strings produced by trailing separators so the count
	// reflects real entries.
	count := 0
	for _, e := range entries {
		if strings.TrimSpace(e) != "" {
			count++
		}
	}
	suffix := plural(count, "entry", "entries")
	return fmt.Sprintf("%d %s in PATH, none match", count, suffix)
}

func init() {
	Register(newShimsInPathCheck())
}
