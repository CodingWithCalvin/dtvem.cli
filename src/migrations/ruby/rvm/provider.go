// Package rvm provides a migration provider for Ruby Version Manager (rvm).
package rvm

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/dtvem/dtvem/src/internal/migration"
)

// Provider implements the migration.Provider interface for rvm.
type Provider struct{}

// NewProvider creates a new rvm migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this version manager.
func (p *Provider) Name() string {
	return "rvm"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "Ruby Version Manager (rvm)"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "ruby"
}

// IsPresent checks if rvm is installed on the system.
func (p *Provider) IsPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	rvmDir := filepath.Join(home, ".rvm", "rubies")
	if _, err := os.Stat(rvmDir); err == nil {
		return true
	}

	return false
}

// DetectVersions finds all versions installed by rvm.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, just return empty list (rvm won't be found anyway)
		return detected, nil //nolint:nilerr // Expected: no home dir means no rvm
	}

	// rvm uses ruby-X.Y.Z format for directory names
	versionRegex := regexp.MustCompile(`^ruby-(\d+\.\d+\.\d+)`)

	rvmDir := filepath.Join(home, ".rvm", "rubies")
	if entries, err := os.ReadDir(rvmDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				matches := versionRegex.FindStringSubmatch(entry.Name())
				if len(matches) >= 2 {
					versionDir := filepath.Join(rvmDir, entry.Name())
					rubyPath := filepath.Join(versionDir, "bin", "ruby")

					if _, err := os.Stat(rubyPath); err == nil {
						detected = append(detected, migration.DetectedVersion{
							Version:   matches[1],
							Path:      rubyPath,
							Source:    "rvm",
							Validated: false,
						})
					}
				}
			}
		}
	}

	return detected, nil
}

// CanAutoUninstall returns true because rvm supports automatic uninstall.
func (p *Provider) CanAutoUninstall() bool {
	return true
}

// UninstallCommand returns the command to uninstall a specific version.
func (p *Provider) UninstallCommand(version string) string {
	return fmt.Sprintf("rvm remove ruby-%s", version)
}

// ManualInstructions returns instructions for manual removal.
func (p *Provider) ManualInstructions() string {
	return "To manually remove an rvm-installed Ruby version:\n" +
		"  1. Run: rvm remove ruby-<version>\n" +
		"  2. Or manually delete the version directory from ~/.rvm/rubies/"
}

// init registers the rvm provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register rvm migration provider: %v", err))
	}
}
