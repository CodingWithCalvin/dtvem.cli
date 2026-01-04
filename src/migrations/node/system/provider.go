// Package system provides a migration provider for system-installed Node.js.
package system

import (
	"os/exec"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/migration"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
)

// Provider implements the migration.Provider interface for system-installed Node.js.
type Provider struct{}

// NewProvider creates a new system Node.js migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this provider.
func (p *Provider) Name() string {
	return "system-node"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "System Node.js"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "node"
}

// IsPresent checks if a system-installed Node.js exists in PATH.
func (p *Provider) IsPresent() bool {
	return path.LookPathExcludingShims("node") != ""
}

// DetectVersions finds system-installed Node.js in PATH.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)

	nodePath := path.LookPathExcludingShims("node")
	if nodePath == "" {
		return detected, nil
	}

	version := p.getVersion(nodePath)
	if version != "" {
		detected = append(detected, migration.DetectedVersion{
			Version:   version,
			Path:      nodePath,
			Source:    "system",
			Validated: true,
		})
	}

	return detected, nil
}

// getVersion runs node --version and extracts the version number.
func (p *Provider) getVersion(execPath string) string {
	cmd := exec.Command(execPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	version := strings.TrimSpace(string(output))
	re := regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)
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
			"  2. Search for Node.js\n" +
			"  3. Click Uninstall\n" +
			"  Or use PowerShell to find and run the uninstaller"
	case "darwin":
		return "To uninstall:\n" +
			"  If installed via Homebrew: brew uninstall node\n" +
			"  If installed via package: check /Applications or use the installer's uninstaller\n" +
			"  Or manually remove from /usr/local/bin/"
	case "linux":
		return "To uninstall:\n" +
			"  If installed via apt: sudo apt remove nodejs\n" +
			"  If installed via yum: sudo yum remove nodejs\n" +
			"  If installed via dnf: sudo dnf remove nodejs"
	default:
		return "Please use your system's package manager to uninstall Node.js"
	}
}

// init registers the system provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		// Ignore registration errors (may already be registered)
	}
}
