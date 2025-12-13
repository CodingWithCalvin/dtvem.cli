// Package pyenv provides a migration provider for pyenv.
package pyenv

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/dtvem/dtvem/src/internal/migration"
)

// Provider implements the migration.Provider interface for pyenv.
type Provider struct{}

// NewProvider creates a new pyenv migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this version manager.
func (p *Provider) Name() string {
	return "pyenv"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "pyenv"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "python"
}

// IsPresent checks if pyenv is installed on the system.
func (p *Provider) IsPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// Check Unix-style pyenv directory
	unixDir := filepath.Join(home, ".pyenv", "versions")
	if _, err := os.Stat(unixDir); err == nil {
		return true
	}

	// Check Windows pyenv directory
	winDir := filepath.Join(home, ".pyenv", "pyenv-win", "versions")
	if _, err := os.Stat(winDir); err == nil {
		return true
	}

	return false
}

// DetectVersions finds all versions installed by pyenv.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, just return empty list (pyenv won't be found anyway)
		return detected, nil //nolint:nilerr // Expected: no home dir means no pyenv
	}

	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+`)

	// Check Unix-style pyenv directory
	pyenvDir := filepath.Join(home, ".pyenv", "versions")
	if entries, err := os.ReadDir(pyenvDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && versionRegex.MatchString(entry.Name()) {
				versionDir := filepath.Join(pyenvDir, entry.Name())

				// Try both Unix and Windows paths
				pythonPaths := []string{
					filepath.Join(versionDir, "bin", "python"),
					filepath.Join(versionDir, "bin", "python3"),
					filepath.Join(versionDir, "python.exe"),
				}

				for _, pythonPath := range pythonPaths {
					if _, err := os.Stat(pythonPath); err == nil {
						detected = append(detected, migration.DetectedVersion{
							Version:   entry.Name(),
							Path:      pythonPath,
							Source:    "pyenv",
							Validated: false,
						})
						break
					}
				}
			}
		}
	}

	// Check Windows pyenv directory
	pyenvWinDir := filepath.Join(home, ".pyenv", "pyenv-win", "versions")
	if entries, err := os.ReadDir(pyenvWinDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && versionRegex.MatchString(entry.Name()) {
				versionDir := filepath.Join(pyenvWinDir, entry.Name())
				pythonPath := filepath.Join(versionDir, "python.exe")

				if _, err := os.Stat(pythonPath); err == nil {
					detected = append(detected, migration.DetectedVersion{
						Version:   entry.Name(),
						Path:      pythonPath,
						Source:    "pyenv",
						Validated: false,
					})
				}
			}
		}
	}

	return detected, nil
}

// CanAutoUninstall returns true because pyenv supports automatic uninstall.
func (p *Provider) CanAutoUninstall() bool {
	return true
}

// UninstallCommand returns the command to uninstall a specific version.
func (p *Provider) UninstallCommand(version string) string {
	return fmt.Sprintf("pyenv uninstall %s", version)
}

// ManualInstructions returns instructions for manual removal.
func (p *Provider) ManualInstructions() string {
	return "To manually remove a pyenv-installed Python version:\n" +
		"  1. Run: pyenv uninstall <version>\n" +
		"  2. Or manually delete the version directory from ~/.pyenv/versions/"
}

// init registers the pyenv provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register pyenv migration provider: %v", err))
	}
}
