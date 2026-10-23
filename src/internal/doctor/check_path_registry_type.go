package doctor

import (
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
)

// systemPathRepairCommand rewrites the System PATH as REG_EXPAND_SZ with its
// entries unchanged. It needs an elevated PowerShell, so doctor only shows it.
const systemPathRepairCommand = `$k = [Microsoft.Win32.Registry]::LocalMachine.OpenSubKey('SYSTEM\CurrentControlSet\Control\Session Manager\Environment', $true); ` +
	`$k.SetValue('Path', $k.GetValue('Path', '', 'DoNotExpandEnvironmentNames'), 'ExpandString')`

// pathRegistryTypeCheck flags a Windows PATH value that holds %VAR%
// references but is stored as REG_SZ. Windows only expands references in a
// REG_EXPAND_SZ value, so entries such as %SystemRoot%\system32 or
// %USERPROFILE%\.dotnet\tools stay literal and their directories are
// missing from PATH.
//
// dtvem releases before #295 wrote PATH as REG_SZ from `dtvem init` and the
// stale-shims fix, and .NET's Environment.SetEnvironmentVariable does too,
// so a converted value may already be in place.
//
// Rewriting the User PATH is safe to automate. The System PATH needs an
// elevated terminal, so for it doctor shows the command instead.
type pathRegistryTypeCheck struct {
	readPaths  func() ([]path.RegistryPath, error)
	repairUser func() error
}

func newPathRegistryTypeCheck() *pathRegistryTypeCheck {
	return &pathRegistryTypeCheck{
		readPaths:  path.RegistryPaths,
		repairUser: path.RepairUserPathType,
	}
}

func (pathRegistryTypeCheck) Name() string { return "path-registry-type" }

func (c pathRegistryTypeCheck) Run() Finding {
	paths, err := c.readPaths()
	if err != nil {
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not read PATH from the registry",
			Details:    []Detail{{Key: "Error", Value: err.Error()}},
			Resolution: "Check that the Environment registry keys are readable.",
		}
	}
	if len(paths) == 0 {
		return Finding{OK: true, Title: "PATH is not stored in the registry on this platform"}
	}

	var details []Detail
	userUnexpanded, systemUnexpanded := false, false
	for _, p := range paths {
		if !p.Unexpanded() {
			continue
		}
		details = append(details, Detail{
			Key:   p.Scope + " PATH",
			Value: "stored as REG_SZ, so its %VAR% entries are not expanded",
		})
		switch p.Scope {
		case "User":
			userUnexpanded = true
		case "System":
			systemUnexpanded = true
		}
	}
	if len(details) == 0 {
		return Finding{OK: true, Title: "PATH registry values expand their %VAR% entries"}
	}

	var resolution []string
	var fix func() error
	if userUnexpanded {
		resolution = append(resolution, "Rewrite the User PATH as REG_EXPAND_SZ; its entries stay as they are.")
		fix = c.repairUser
	}
	if systemUnexpanded {
		resolution = append(resolution,
			"The System PATH needs an elevated PowerShell:",
			"  "+systemPathRepairCommand)
	}
	resolution = append(resolution, "Restart your terminal afterwards.")

	return Finding{
		Severity:   SeverityWarning,
		Title:      "PATH entries that use %VAR% are not being expanded",
		Details:    details,
		Resolution: strings.Join(resolution, "\n"),
		Fix:        fix,
	}
}

func init() {
	Register(newPathRegistryTypeCheck())
}
