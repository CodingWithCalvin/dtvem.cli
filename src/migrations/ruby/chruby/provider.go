// Package chruby provides a migration provider for chruby.
package chruby

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/dtvem/dtvem/src/internal/migration"
)

// Provider implements the migration.Provider interface for chruby.
type Provider struct{}

// NewProvider creates a new chruby migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this version manager.
func (p *Provider) Name() string {
	return "chruby"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "chruby"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "ruby"
}

// IsPresent checks if chruby is installed on the system.
func (p *Provider) IsPresent() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	// chruby looks in /opt/rubies and ~/.rubies
	chrubyDirs := []string{
		"/opt/rubies",
		filepath.Join(home, ".rubies"),
	}

	for _, chrubyDir := range chrubyDirs {
		if _, err := os.Stat(chrubyDir); err == nil {
			return true
		}
	}

	return false
}

// DetectVersions finds all versions installed for chruby.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		// If we can't get home dir, just return empty list (chruby won't be found anyway)
		return detected, nil //nolint:nilerr // Expected: no home dir means no chruby
	}

	// chruby uses ruby-X.Y.Z format for directory names
	versionRegex := regexp.MustCompile(`^ruby-(\d+\.\d+\.\d+)`)

	// chruby looks in /opt/rubies and ~/.rubies
	chrubyDirs := []string{
		"/opt/rubies",
		filepath.Join(home, ".rubies"),
	}

	for _, chrubyDir := range chrubyDirs {
		if entries, err := os.ReadDir(chrubyDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					matches := versionRegex.FindStringSubmatch(entry.Name())
					if len(matches) >= 2 {
						versionDir := filepath.Join(chrubyDir, entry.Name())
						rubyPath := filepath.Join(versionDir, "bin", "ruby")

						if _, err := os.Stat(rubyPath); err == nil {
							detected = append(detected, migration.DetectedVersion{
								Version:   matches[1],
								Path:      rubyPath,
								Source:    "chruby",
								Validated: false,
							})
						}
					}
				}
			}
		}
	}

	return detected, nil
}

// CanAutoUninstall returns false because chruby doesn't have a built-in uninstall command.
func (p *Provider) CanAutoUninstall() bool {
	return false
}

// UninstallCommand returns an empty string because chruby doesn't have automatic uninstall.
func (p *Provider) UninstallCommand(version string) string {
	return ""
}

// ManualInstructions returns instructions for manual removal.
func (p *Provider) ManualInstructions() string {
	return "To manually remove a chruby-installed Ruby version:\n" +
		"  1. Delete the version directory from ~/.rubies/ or /opt/rubies/\n" +
		"  2. For example: rm -rf ~/.rubies/ruby-<version>\n" +
		"Note: If using ruby-install, you may want to keep track of installed versions"
}

// init registers the chruby provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register chruby migration provider: %v", err))
	}
}
