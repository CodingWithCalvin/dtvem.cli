// Package nvm provides a migration provider for Node Version Manager (nvm).
package nvm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/migration"
)

// Provider implements the migration.Provider interface for nvm.
type Provider struct{}

// NewProvider creates a new nvm migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this version manager.
func (p *Provider) Name() string {
	return "nvm"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "Node Version Manager (nvm)"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "node"
}

// IsPresent checks if nvm is installed on the system.
func (p *Provider) IsPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Check Unix-style nvm directory
	unixDir := filepath.Join(home, ".nvm", "versions", "node")
	if _, err := os.Stat(unixDir); err == nil {
		return true
	}

	// Check Windows nvm directory
	winDir := filepath.Join(home, "AppData", "Roaming", "nvm")
	if _, err := os.Stat(winDir); err == nil {
		return true
	}

	return false
}

// DetectVersions finds all versions installed by nvm.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, just return empty list (nvm won't be found anyway)
		return detected, nil //nolint:nilerr // Expected: no home dir means no nvm
	}

	// Check Unix-style nvm directory
	nvmDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				versionDir := filepath.Join(nvmDir, entry.Name())
				nodePath := filepath.Join(versionDir, "bin", "node")

				if _, err := os.Stat(nodePath); err == nil {
					// Extract version from directory name (e.g., "v22.0.0" -> "22.0.0")
					version := strings.TrimPrefix(entry.Name(), "v")

					detected = append(detected, migration.DetectedVersion{
						Version:   version,
						Path:      nodePath,
						Source:    "nvm",
						Validated: false,
					})
				}
			}
		}
	}

	// Check Windows nvm directory
	nvmWinDir := filepath.Join(home, "AppData", "Roaming", "nvm")
	if entries, err := os.ReadDir(nvmWinDir); err == nil {
		versionRegex := regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
		for _, entry := range entries {
			if entry.IsDir() && versionRegex.MatchString(entry.Name()) {
				versionDir := filepath.Join(nvmWinDir, entry.Name())
				nodePath := filepath.Join(versionDir, "node.exe")

				if _, err := os.Stat(nodePath); err == nil {
					version := strings.TrimPrefix(entry.Name(), "v")

					detected = append(detected, migration.DetectedVersion{
						Version:   version,
						Path:      nodePath,
						Source:    "nvm",
						Validated: false,
					})
				}
			}
		}
	}

	return detected, nil
}

// CanAutoUninstall returns true because nvm supports automatic uninstall.
func (p *Provider) CanAutoUninstall() bool {
	return true
}

// UninstallCommand returns the command to uninstall a specific version.
func (p *Provider) UninstallCommand(version string) string {
	return fmt.Sprintf("nvm uninstall %s", version)
}

// ManualInstructions returns instructions for manual removal.
func (p *Provider) ManualInstructions() string {
	return "To manually remove an nvm-installed Node.js version:\n" +
		"  1. Run: nvm uninstall <version>\n" +
		"  2. Or manually delete the version directory from ~/.nvm/versions/node/"
}

// init registers the nvm provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register nvm migration provider: %v", err))
	}
}
