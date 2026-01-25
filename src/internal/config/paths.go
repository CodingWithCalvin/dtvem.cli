// Package config manages dtvem configuration including paths and version resolution
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// Paths holds all important dtvem directory paths
type Paths struct {
	Root     string // Root dtvem directory (~/.dtvem)
	Shims    string // Shims directory (~/.dtvem/shims)
	Versions string // Versions directory (~/.dtvem/versions)
	Config   string // Config directory (~/.dtvem/config)
	Cache    string // Cache directory (~/.dtvem/cache)
}

var (
	defaultPaths *Paths
	pathsOnce    sync.Once
)

// DefaultPaths returns the default dtvem paths.
// This function is thread-safe and guarantees single initialization.
func DefaultPaths() *Paths {
	pathsOnce.Do(func() {
		defaultPaths = initPaths()
	})
	return defaultPaths
}

// initPaths initializes the default paths
func initPaths() *Paths {
	root := getRootDir()
	return &Paths{
		Root:     root,
		Shims:    filepath.Join(root, "shims"),
		Versions: filepath.Join(root, "versions"),
		Config:   filepath.Join(root, "config"),
		Cache:    filepath.Join(root, "cache"),
	}
}

// getRootDir returns the root dtvem directory based on platform conventions.
//
// Path Selection Rationale:
//
// Linux: Follows XDG Base Directory Specification (https://specifications.freedesktop.org/basedir-spec/)
//   - Uses $XDG_DATA_HOME/dtvem if XDG_DATA_HOME is set
//   - Otherwise uses ~/.local/share/dtvem (XDG default)
//   - This is the standard location for user-specific data files on Linux
//
// macOS: Uses ~/.dtvem by default
//   - macOS has its own conventions (~/Library/Application Support) but many CLI tools
//     use dotfiles in home directory for better discoverability and Unix compatibility
//   - ~/.dtvem is more familiar to users coming from tools like nvm, pyenv, rbenv
//   - Optionally respects $XDG_DATA_HOME if set (for users who prefer XDG consistency)
//
// Windows: Uses %USERPROFILE%\.dtvem by default
//   - Alternatives considered: %LOCALAPPDATA% (C:\Users\<user>\AppData\Local)
//   - Chose home directory for consistency with macOS/Linux and better visibility
//   - Users expect CLI tool configs in their home directory
//   - Easier to locate and backup than buried in AppData
//   - Optionally respects $XDG_DATA_HOME if set (for users who prefer XDG consistency)
//
// Override: DTVEM_ROOT environment variable overrides all platform defaults
func getRootDir() string {
	// Check for DTVEM_ROOT environment variable first (overrides all)
	if root := os.Getenv("DTVEM_ROOT"); root != "" {
		return root
	}

	// Use home directory
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home is not available
		return ".dtvem"
	}

	// On Linux, always respect XDG Base Directory specification
	if runtime.GOOS == constants.OSLinux {
		return getXDGDataPath(home)
	}

	// On macOS and Windows, use XDG_DATA_HOME if explicitly set (opt-in)
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "dtvem")
	}

	// Default for macOS and Windows: ~/.dtvem
	return filepath.Join(home, ".dtvem")
}

// getXDGDataPath returns the XDG-compliant data path for dtvem on Linux
// Uses XDG_DATA_HOME if set, otherwise defaults to ~/.local/share/dtvem
func getXDGDataPath(home string) string {
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "dtvem")
	}
	// XDG default: ~/.local/share
	return filepath.Join(home, ".local", "share", "dtvem")
}

// RuntimeVersionPath returns the path to a specific runtime version
func RuntimeVersionPath(runtimeName, version string) string {
	paths := DefaultPaths()
	return filepath.Join(paths.Versions, runtimeName, version)
}

// GlobalConfigPath returns the path to the global config file
func GlobalConfigPath() string {
	paths := DefaultPaths()
	return filepath.Join(paths.Config, RuntimesFileName)
}

// ShimPath returns the path to a specific shim executable
func ShimPath(shimName string) string {
	paths := DefaultPaths()
	// Add .exe extension on Windows
	if runtime.GOOS == constants.OSWindows {
		shimName = shimName + constants.ExtExe
	}
	return filepath.Join(paths.Shims, shimName)
}

// EnsureDirectories creates all necessary dtvem directories
func EnsureDirectories() error {
	paths := DefaultPaths()
	dirs := []string{
		paths.Root,
		paths.Shims,
		paths.Versions,
		paths.Config,
		paths.Cache,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// LocalConfigDir returns the local .dtvem directory path for the current working directory
func LocalConfigDir() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return filepath.Join(cwd, LocalConfigDirName)
}

// LocalConfigPath returns the path to the local runtimes.json file
func LocalConfigPath() string {
	return filepath.Join(LocalConfigDir(), RuntimesFileName)
}

// LocalConfigDirName is the name of the local .dtvem directory
const LocalConfigDirName = ".dtvem"

// RuntimesFileName is the name of the runtimes configuration file
const RuntimesFileName = "runtimes.json"

// ShimMapFileName is the name of the shim-to-runtime mapping cache file
const ShimMapFileName = "shim-map.json"

// ShimMapPath returns the path to the shim-to-runtime mapping cache file
func ShimMapPath() string {
	paths := DefaultPaths()
	return filepath.Join(paths.Cache, ShimMapFileName)
}

// ResetPathsCache resets the cached paths, forcing reinitialization on next access.
// This is primarily useful for testing.
func ResetPathsCache() {
	pathsOnce = sync.Once{}
	defaultPaths = nil
}
