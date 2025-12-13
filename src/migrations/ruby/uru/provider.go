// Package uru provides a migration provider for uru (multi-platform Ruby version manager).
package uru

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	goruntime "runtime"

	"github.com/dtvem/dtvem/src/internal/migration"
)

// rubyEntry represents a Ruby installation in uru's rubies.json.
type rubyEntry struct {
	ID          string `json:"ID"`
	TagLabel    string `json:"TagLabel"`
	Exe         string `json:"Exe"`
	Home        string `json:"Home"`
	GemHome     string `json:"GemHome"`
	Description string `json:"Description"`
}

// rubiesJSON represents the structure of uru's rubies.json file.
type rubiesJSON struct {
	Version string               `json:"Version"`
	Rubies  map[string]rubyEntry `json:"Rubies"`
}

// Provider implements the migration.Provider interface for uru.
type Provider struct{}

// NewProvider creates a new uru migration provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the identifier for this version manager.
func (p *Provider) Name() string {
	return "uru"
}

// DisplayName returns the human-readable name.
func (p *Provider) DisplayName() string {
	return "uru"
}

// Runtime returns the runtime this provider manages.
func (p *Provider) Runtime() string {
	return "ruby"
}

// getUruHome returns the uru home directory.
// It checks URU_HOME environment variable first, then falls back to ~/.uru.
func (p *Provider) getUruHome() string {
	if uruHome := os.Getenv("URU_HOME"); uruHome != "" {
		return uruHome
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, ".uru")
}

// IsPresent checks if uru is installed on the system.
func (p *Provider) IsPresent() bool {
	uruHome := p.getUruHome()
	if uruHome == "" {
		return false
	}

	rubiesPath := filepath.Join(uruHome, "rubies.json")
	if _, err := os.Stat(rubiesPath); err == nil {
		return true
	}

	return false
}

// DetectVersions finds all versions registered with uru.
func (p *Provider) DetectVersions() ([]migration.DetectedVersion, error) {
	detected := make([]migration.DetectedVersion, 0)

	uruHome := p.getUruHome()
	if uruHome == "" {
		return detected, nil
	}

	rubiesPath := filepath.Join(uruHome, "rubies.json")
	data, err := os.ReadFile(rubiesPath)
	if err != nil {
		// If we can't read the file, just return empty list
		return detected, nil //nolint:nilerr // Expected: no rubies.json means no uru rubies
	}

	var rubies rubiesJSON
	if err := json.Unmarshal(data, &rubies); err != nil {
		// Invalid JSON, return empty list
		return detected, nil //nolint:nilerr // Expected: invalid JSON means no usable uru data
	}

	// Version pattern: major.minor.patch (e.g., "3.2.0")
	versionRegex := regexp.MustCompile(`(\d+\.\d+\.\d+)`)

	for tag, entry := range rubies.Rubies {
		if entry.Home == "" {
			continue
		}

		// Extract version from ID field (e.g., "3.2.0-p0" -> "3.2.0")
		version := ""
		if matches := versionRegex.FindStringSubmatch(entry.ID); len(matches) >= 2 {
			version = matches[1]
		}

		if version == "" {
			continue
		}

		// Build path to ruby executable
		rubyExe := "ruby"
		if goruntime.GOOS == "windows" {
			rubyExe = "ruby.exe"
		}
		rubyPath := filepath.Join(entry.Home, rubyExe)

		// Verify the executable exists
		if _, err := os.Stat(rubyPath); err != nil {
			continue
		}

		detected = append(detected, migration.DetectedVersion{
			Version:   version,
			Path:      rubyPath,
			Source:    fmt.Sprintf("uru (%s)", tag),
			Validated: false,
		})
	}

	return detected, nil
}

// CanAutoUninstall returns true because uru supports removing registered rubies.
func (p *Provider) CanAutoUninstall() bool {
	return true
}

// UninstallCommand returns the command to remove a Ruby from uru's registry.
func (p *Provider) UninstallCommand(version string) string {
	return fmt.Sprintf("uru admin rm %s", version)
}

// ManualInstructions returns instructions for manual removal.
func (p *Provider) ManualInstructions() string {
	return "To remove a Ruby from uru's registry:\n" +
		"  1. Run: uru admin rm <tag>\n" +
		"  2. This only removes uru's reference, not the Ruby installation itself\n" +
		"  3. To fully uninstall, also remove the Ruby directory manually"
}

// init registers the uru provider on package load.
func init() {
	if err := migration.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register uru migration provider: %v", err))
	}
}
