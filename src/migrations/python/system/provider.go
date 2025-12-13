// Package system provides a migration provider for system-installed Python.
package system

import (
	"os/exec"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/dtvem/dtvem/src/internal/migration"
	"github.com/dtvem/dtvem/src/internal/path"
)

// Provider implements the migration.Provider interface for system-installed Python.
type Provider struct{}

// NewProvider creates a new system Python migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this provider.
func (p *Provider) Name() string {
	return "system-python"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "System Python"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "python"
}

// IsPresent checks if a system-installed Python exists in PATH.
func (p *Provider) IsPresent() bool {
	for _, cmd := range []string{"python3", "python"} {
		if path.LookPathExcludingShims(cmd) != "" {
			return true
		}
	}
	return false
}

// DetectVersions finds system-installed Python in PATH.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)
	seen := make(map[string]bool)

	for _, cmd := range []string{"python3", "python"} {
		pythonPath := path.LookPathExcludingShims(cmd)
		if pythonPath == "" || seen[pythonPath] {
			continue
		}
		seen[pythonPath] = true

		version := p.getVersion(pythonPath)
		if version != "" {
			detected = append(detected, migration.DetectedVersion{
				Version:   version,
				Path:      pythonPath,
				Source:    "system",
				Validated: true,
			})
		}
	}

	return detected, nil
}

// getVersion runs python --version and extracts the version number.
func (p *Provider) getVersion(execPath string) string {
	cmd := exec.Command(execPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	version := strings.TrimSpace(string(output))
	re := regexp.MustCompile(`Python\s+(\d+\.\d+\.\d+)`)
	matches := re.FindStringSubmatch(version)
	if len(matches) >= 2 {
		return matches[1]
	}

	return ""
}

// CanAutoUninstall returns false because system installs require manual removal.
func (p *Provider) CanAutoUninstall() bool {
	return false
}

// UninstallCommand returns an empty string because system installs need manual removal.
func (p *Provider) UninstallCommand(version string) string {
	return ""
}

// ManualInstructions returns OS-specific uninstall instructions.
func (p *Provider) ManualInstructions() string {
	switch goruntime.GOOS {
	case "windows":
		return "To uninstall:\n" +
			"  1. Open Settings \u2192 Apps \u2192 Installed apps\n" +
			"  2. Search for Python\n" +
			"  3. Click Uninstall\n" +
			"  Or use PowerShell to find and run the uninstaller"
	case "darwin":
		return "To uninstall:\n" +
			"  If installed via Homebrew: brew uninstall python\n" +
			"  If installed via package: check /Applications or use the installer's uninstaller\n" +
			"  Or manually remove from /usr/local/bin/"
	case "linux":
		return "To uninstall:\n" +
			"  If installed via apt: sudo apt remove python3\n" +
			"  If installed via yum: sudo yum remove python3\n" +
			"  If installed via dnf: sudo dnf remove python3"
	default:
		return "Please use your system's package manager to uninstall Python"
	}
}

// init registers the system provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		// Ignore registration errors (may already be registered)
	}
}
