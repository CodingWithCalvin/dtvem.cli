// Package migration provides a plugin-style architecture for migrating from
// other version managers (nvm, pyenv, rbenv, etc.) to dtvem.
package migration

// Provider defines the interface that all migration providers must implement.
// Each provider handles detection and cleanup for a specific version manager.
type Provider interface {
	// Name returns the identifier for this version manager (e.g., "nvm", "pyenv", "rbenv")
	Name() string

	// DisplayName returns the human-readable name (e.g., "Node Version Manager (nvm)")
	DisplayName() string

	// Runtime returns the runtime this provider manages (e.g., "node", "python", "ruby")
	Runtime() string

	// IsPresent checks if this version manager is installed on the system
	IsPresent() bool

	// DetectVersions finds all versions installed by this version manager
	DetectVersions() ([]DetectedVersion, error)

	// CanAutoUninstall returns true if versions can be uninstalled automatically
	CanAutoUninstall() bool

	// UninstallCommand returns the command to uninstall a specific version
	// Returns empty string if automatic uninstall is not supported
	UninstallCommand(version string) string

	// ManualInstructions returns instructions for manual removal
	// Used when automatic uninstall is not available
	ManualInstructions() string
}

// DetectedVersion represents a runtime version found by a migration provider.
type DetectedVersion struct {
	Version   string // Version string (e.g., "22.0.0", "3.11.0")
	Path      string // Path to the executable
	Source    string // Source/version manager name (e.g., "nvm", "pyenv")
	Validated bool   // Whether we've verified this version works
}

// String returns a formatted string representation
func (dv DetectedVersion) String() string {
	return "v" + dv.Version + " (" + dv.Source + ") " + dv.Path
}
