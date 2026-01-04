// Package rbenv provides a migration provider for rbenv.
package rbenv

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/migration"
)

// Provider implements the migration.Provider interface for rbenv.
type Provider struct{}

// NewProvider creates a new rbenv migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this version manager.
func (p *Provider) Name() string {
	return "rbenv"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "rbenv"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "ruby"
}

// IsPresent checks if rbenv is installed on the system.
func (p *Provider) IsPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	rbenvDir := filepath.Join(home, ".rbenv", "versions")
	if _, err := os.Stat(rbenvDir); err == nil {
		return true
	}

	return false
}

// DetectVersions finds all versions installed by rbenv.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, just return empty list (rbenv won't be found anyway)
		return detected, nil //nolint:nilerr // Expected: no home dir means no rbenv
	}

	versionRegex := regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	rbenvDir := filepath.Join(home, ".rbenv", "versions")
	if entries, err := os.ReadDir(rbenvDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() && versionRegex.MatchString(entry.Name()) {
				versionDir := filepath.Join(rbenvDir, entry.Name())
				rubyPath := filepath.Join(versionDir, "bin", "ruby")

				if _, err := os.Stat(rubyPath); err == nil {
					detected = append(detected, migration.DetectedVersion{
						Version:   entry.Name(),
						Path:      rubyPath,
						Source:    "rbenv",
						Validated: false,
					})
				}
			}
		}
	}

	return detected, nil
}

// CanAutoUninstall returns true because rbenv supports automatic uninstall.
func (p *Provider) CanAutoUninstall() bool {
	return true
}

// UninstallCommand returns the command to uninstall a specific version.
func (p *Provider) UninstallCommand(version string) string {
	return fmt.Sprintf("rbenv uninstall %s", version)
}

// ManualInstructions returns instructions for manual removal.
func (p *Provider) ManualInstructions() string {
	return "To manually remove an rbenv-installed Ruby version:\n" +
		"  1. Run: rbenv uninstall <version>\n" +
		"  2. Or manually delete the version directory from ~/.rbenv/versions/"
}

// init registers the rbenv provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register rbenv migration provider: %v", err))
	}
}
