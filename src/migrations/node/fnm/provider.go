// Package fnm provides a migration provider for Fast Node Manager (fnm).
package fnm

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/migration"
)

// Provider implements the migration.Provider interface for fnm.
type Provider struct{}

// NewProvider creates a new fnm migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this version manager.
func (p *Provider) Name() string {
	return "fnm"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "Fast Node Manager (fnm)"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "node"
}

// IsPresent checks if fnm is installed on the system.
func (p *Provider) IsPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// fnm stores versions in various locations
	fnmDirs := []string{
		filepath.Join(home, ".local", "share", "fnm", "node-versions"),
		filepath.Join(home, ".fnm", "node-versions"),
		filepath.Join(home, "Library", "Application Support", "fnm", "node-versions"), // macOS
	}

	for _, fnmDir := range fnmDirs {
		if _, err := os.Stat(fnmDir); err == nil {
			return true
		}
	}

	return false
}

// DetectVersions finds all versions installed by fnm.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, just return empty list (fnm won't be found anyway)
		return detected, nil //nolint:nilerr // Expected: no home dir means no fnm
	}

	// fnm stores versions in ~/.local/share/fnm/node-versions or similar
	fnmDirs := []string{
		filepath.Join(home, ".local", "share", "fnm", "node-versions"),
		filepath.Join(home, ".fnm", "node-versions"),
		filepath.Join(home, "Library", "Application Support", "fnm", "node-versions"), // macOS
	}

	for _, fnmDir := range fnmDirs {
		if entries, err := os.ReadDir(fnmDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					versionDir := filepath.Join(fnmDir, entry.Name())

					// Try both Unix and Windows paths
					nodePaths := []string{
						filepath.Join(versionDir, "installation", "bin", "node"),
						filepath.Join(versionDir, "installation", "node.exe"),
						filepath.Join(versionDir, "bin", "node"),
						filepath.Join(versionDir, "node.exe"),
					}

					for _, nodePath := range nodePaths {
						if _, err := os.Stat(nodePath); err == nil {
							version := strings.TrimPrefix(entry.Name(), "v")

							detected = append(detected, migration.DetectedVersion{
								Version:   version,
								Path:      nodePath,
								Source:    "fnm",
								Validated: false,
							})
							break
						}
					}
				}
			}
		}
	}

	return detected, nil
}

// CanAutoUninstall returns true because fnm supports automatic uninstall.
func (p *Provider) CanAutoUninstall() bool {
	return true
}

// UninstallCommand returns the command to uninstall a specific version.
func (p *Provider) UninstallCommand(version string) string {
	return fmt.Sprintf("fnm uninstall %s", version)
}

// ManualInstructions returns instructions for manual removal.
func (p *Provider) ManualInstructions() string {
	return "To manually remove an fnm-installed Node.js version:\n" +
		"  1. Run: fnm uninstall <version>\n" +
		"  2. Or manually delete the version directory from ~/.local/share/fnm/node-versions/"
}

// init registers the fnm provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register fnm migration provider: %v", err))
	}
}
