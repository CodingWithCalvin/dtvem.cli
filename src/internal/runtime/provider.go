// Package runtime defines the provider interface and registry for runtime managers
package runtime

// ShimProvider defines the minimal interface needed by the shim executable.
// This interface excludes heavy operations like Install() and ListAvailable()
// that require net/http and other dependencies not needed for shim execution.
type ShimProvider interface {
	// Name returns the name of the runtime (e.g., "python", "node", "ruby")
	Name() string

	// DisplayName returns a human-readable name (e.g., "Python", "Node.js", "Ruby")
	DisplayName() string

	// Shims returns the list of shim executable names this runtime provides
	// For example, Python returns ["python", "python3", "pip", "pip3"]
	// Node.js returns ["node", "npm", "npx"]
	Shims() []string

	// ExecutablePath returns the path to the main executable for a given version
	// For example, for Python 3.11.0, this might return "/path/to/python3.11"
	ExecutablePath(version string) (string, error)

	// IsInstalled checks if a specific version is installed
	IsInstalled(version string) (bool, error)

	// ShouldReshimAfter checks if the given command should trigger a reshim.
	// Returns true if the command installs or uninstalls global packages that add/remove executables.
	// The shimName parameter indicates which shim was invoked (e.g., "npm", "pip")
	// The args parameter contains the command arguments (e.g., ["install", "-g", "typescript"])
	ShouldReshimAfter(shimName string, args []string) bool

	// GetEnvironment returns environment variables that should be set when executing
	// this runtime's binaries. For example, Ruby needs LD_LIBRARY_PATH set to find libruby.so.
	// Returns an empty map if no special environment is needed.
	GetEnvironment(version string) (map[string]string, error)
}

// Provider defines the full interface that all runtime providers must implement.
// It embeds ShimProvider and adds operations that require heavier dependencies.
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
