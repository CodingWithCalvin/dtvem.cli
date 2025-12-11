// Package config manages dtvem configuration including paths and version resolution
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/dtvem/dtvem/src/internal/constants"
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

// getRootDir returns the root dtvem directory
func getRootDir() string {
	// Check for DTVEM_ROOT environment variable first
	if root := os.Getenv("DTVEM_ROOT"); root != "" {
		return root
	}

	// Use home directory
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home is not available
		return ".dtvem"
	}

	return filepath.Join(home, ".dtvem")
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
