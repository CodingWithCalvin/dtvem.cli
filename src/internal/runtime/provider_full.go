//go:build !shim

package runtime

// Provider defines the full interface that all runtime providers must implement.
// It embeds ShimProvider and adds operations that require heavier dependencies
// (HTTP, archive extraction, manifest fetching). These methods are not compiled
// into the shim binary; in shim builds, Provider is an alias for ShimProvider
// (see provider_shim.go).
type Provider interface {
	ShimProvider

	// Install downloads and installs a specific version of the runtime
	Install(version string) error

	// Uninstall removes an installed version of the runtime
	Uninstall(version string) error

	// ListInstalled returns all installed versions of this runtime
	ListInstalled() ([]InstalledVersion, error)

	// ListAvailable returns all available versions that can be installed
	// This might query online sources or use cached data
	ListAvailable() ([]AvailableVersion, error)

	// InstallPath returns the installation directory for a given version
	InstallPath(version string) (string, error)

	// GlobalVersion returns the globally configured version, if any
	GlobalVersion() (string, error)

	// SetGlobalVersion sets the global default version
	SetGlobalVersion(version string) error

	// LocalVersion returns the locally configured version for the current directory
	// This reads from dtvem.config.json
	LocalVersion() (string, error)

	// SetLocalVersion sets the local version for the current directory
	SetLocalVersion(version string) error

	// CurrentVersion returns the currently active version
	// (checks local first, then global)
	CurrentVersion() (string, error)

	// DetectInstalled scans the system for existing installations of this runtime
	// Returns a list of detected versions with their paths and sources
	DetectInstalled() ([]DetectedVersion, error)

	// GlobalPackages detects globally installed packages for a specific installation
	// Takes the installation path and returns a list of package names
	// Returns empty slice if the runtime doesn't support global packages
	GlobalPackages(installPath string) ([]string, error)

	// InstallGlobalPackages reinstalls global packages to a specific version
	// Takes the version and list of package names to install
	// Returns nil if the runtime doesn't support global packages
	InstallGlobalPackages(version string, packages []string) error

	// ManualPackageInstallCommand returns the command string for manually installing packages
	// Used to provide help text to users if automatic package installation fails
	// Returns empty string if the runtime doesn't support global packages
	ManualPackageInstallCommand(packages []string) string
}
