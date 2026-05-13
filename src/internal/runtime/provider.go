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
