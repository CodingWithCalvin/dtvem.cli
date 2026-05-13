package doctor

import (
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// shimBinaryCheck verifies that the dtvem-shim helper exists alongside
// the running dtvem binary. The shim binary is what gets copied into
// ~/.dtvem/shims/<command>.exe — without it, `dtvem reshim` can't
// recreate any shims, and a fresh install can't lay down its initial
// set.
//
// The fix is to reinstall dtvem (the install scripts always ship both
// binaries together), so this check is manual: we don't try to fetch
// or rebuild the shim binary from inside dtvem itself.
type shimBinaryCheck struct {
	// executable is the function used to resolve the running dtvem
	// binary's path. Defaults to os.Executable; tests inject a stub so
	// the check can exercise both the present and missing branches
	// without having to lay down a real binary next to the test runner.
	executable func() (string, error)
}

func newShimBinaryCheck() *shimBinaryCheck {
	return &shimBinaryCheck{executable: os.Executable}
}

func (shimBinaryCheck) Name() string { return "shim-binary-present" }

func (c shimBinaryCheck) Run() Finding {
	exeFn := c.executable
	if exeFn == nil {
		exeFn = os.Executable
	}
	exe, err := exeFn()
	if err != nil {
		// If we can't even introspect our own location, we can't
		// answer this question; report it rather than guessing.
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not locate dtvem executable for shim-binary check",
			Details:    []Detail{{Key: "Error", Value: err.Error()}},
			Resolution: "This is unusual — please report it at https://github.com/CodingWithCalvin/dtvem.cli/issues",
		}
	}

	shimName := "dtvem-shim"
	if goruntime.GOOS == constants.OSWindows {
		shimName = "dtvem-shim" + constants.ExtExe
	}
	shimPath := filepath.Join(filepath.Dir(exe), shimName)

	if info, err := os.Stat(shimPath); err == nil && !info.IsDir() {
		return Finding{OK: true, Title: "dtvem-shim binary is present"}
	}

	return Finding{
		Severity: SeverityError,
		Title:    "dtvem-shim binary is missing",
		Details: []Detail{
			{Key: "Expected at", Value: shimPath},
		},
		Resolution: shimBinaryResolution(),
	}
}

// shimBinaryResolution returns the platform-specific reinstall command.
// We point at the same installers used during initial setup so users
// don't need to discover release artifact URLs themselves.
func shimBinaryResolution() string {
	if goruntime.GOOS == constants.OSWindows {
		return "Reinstall dtvem to restore the shim binary:\n  irm dtvem.io/install.ps1 | iex"
	}
	return "Reinstall dtvem to restore the shim binary:\n  curl -fsSL dtvem.io/install.sh | bash"
}

func init() {
	Register(newShimBinaryCheck())
}
