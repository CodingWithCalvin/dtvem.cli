// Package shim manages shim executables that intercept runtime commands
package shim

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	runtimepkg "github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// Manager handles shim creation and management
type Manager struct {
	shimSource string // Path to the shim executable
}

// NewManager creates a new shim manager
func NewManager() (*Manager, error) {
	// Find the shim executable
	// It should be in the same directory as the dtvem executable
	// Or we'll build it on demand
	shimSource, err := findShimExecutable()
	if err != nil {
		return nil, fmt.Errorf("could not find shim executable: %w", err)
	}

	return &Manager{
		shimSource: shimSource,
	}, nil
}

// findShimExecutable locates the shim executable
func findShimExecutable() (string, error) {
	// Get the directory where dtvem is installed
	execPath, err := os.Executable()
	if err != nil {
		return "", err
	}

	execDir := filepath.Dir(execPath)

	// Look for dtvem-shim or dtvem-shim.exe in the same directory
	shimName := "dtvem-shim"
	if runtime.GOOS == constants.OSWindows {
		shimName = "dtvem-shim.exe"
	}

	shimPath := filepath.Join(execDir, shimName)

	// Check if it exists
	if _, err := os.Stat(shimPath); err == nil {
		return shimPath, nil
	}

	// If not found, we'll need to build it
	// For now, return an error
	return "", fmt.Errorf("shim executable not found at %s", shimPath)
}

// CreateShim creates a shim for the given executable name
func (m *Manager) CreateShim(shimName string) error {
	shimPath := config.ShimPath(shimName)

	// Copy the shim executable to the new location
	if err := copyFile(m.shimSource, shimPath); err != nil {
		return fmt.Errorf("failed to create shim %s: %w", shimName, err)
	}

	// Make it executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(shimPath, 0755); err != nil {
			return fmt.Errorf("failed to make shim executable: %w", err)
		}
	}

	return nil
}

// CreateShims creates multiple shims at once
func (m *Manager) CreateShims(shimNames []string) error {
	for _, shimName := range shimNames {
		if err := m.CreateShim(shimName); err != nil {
			return err
		}
	}
	return nil
}

// RemoveShim removes a shim
func (m *Manager) RemoveShim(shimName string) error {
	shimPath := config.ShimPath(shimName)

	if err := os.Remove(shimPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove shim %s: %w", shimName, err)
	}

	return nil
}

// ListShims returns all existing shims
func (m *Manager) ListShims() ([]string, error) {
	paths := config.DefaultPaths()
	shimsDir := paths.Shims

	entries, err := os.ReadDir(shimsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	shims := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			name := entry.Name()
			// Remove .exe extension on Windows for consistency
			if runtime.GOOS == "windows" {
				name = filepath.Base(name)
				name = name[:len(name)-len(filepath.Ext(name))]
			}
			shims = append(shims, name)
		}
	}

	return shims, nil
}

// RehashResult contains the results of a reshim operation
type RehashResult struct {
	// ShimsByRuntime maps runtime names to their shim names
	ShimsByRuntime map[string][]string
	// TotalShims is the total number of shims created
	TotalShims int
}

// RuntimeShimInfo contains shim information for a single runtime
type RuntimeShimInfo struct {
	RuntimeName string
	Shims       []string
}

// RehashCallback is called before processing each runtime
// runtimeName is the internal name, displayName is the user-friendly name
type RehashCallback func(runtimeName, displayName string)

// RehashWithCallback regenerates all shims, calling the callback before each runtime
func (m *Manager) RehashWithCallback(callback RehashCallback) (*RehashResult, error) {
	paths := config.DefaultPaths()
	versionsDir := paths.Versions

	// Read all runtime directories
	entries, err := os.ReadDir(versionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no versions directory found - no runtimes installed yet")
		}
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	// Collect shim-to-runtime mappings (shim name -> runtime name)
	shimMap := make(ShimMap)
	// Also track shims by runtime for reporting
	shimsByRuntime := make(map[string][]string)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		runtimeName := entry.Name()

		// Check if this runtime has any versions installed
		runtimeVersionsDir := filepath.Join(versionsDir, runtimeName)
		versionEntries, err := os.ReadDir(runtimeVersionsDir)
		if err != nil {
			continue
		}

		// Skip if no versions installed
		hasVersions := false
		for _, ve := range versionEntries {
			if ve.IsDir() {
				hasVersions = true
				break
			}
		}
		if !hasVersions {
			continue
		}

		// Get display name from provider
		displayName := runtimeName
		if provider, err := runtimepkg.Get(runtimeName); err == nil {
			displayName = provider.DisplayName()
		}

		// Call the callback before processing this runtime
		if callback != nil {
			callback(runtimeName, displayName)
		}

		// For each installed version, scan for executables
		for _, versionEntry := range versionEntries {
			if !versionEntry.IsDir() {
				continue
			}

			versionDir := filepath.Join(runtimeVersionsDir, versionEntry.Name())

			// First, add core runtime shims (from provider)
			coreShims := RuntimeShims(runtimeName)
			for _, shimName := range coreShims {
				shimMap[shimName] = runtimeName
				shimsByRuntime[runtimeName] = appendUnique(shimsByRuntime[runtimeName], shimName)
			}

			// Then, scan bin directory for globally installed packages
			binDir := filepath.Join(versionDir, "bin")
			if execs, err := findExecutables(binDir); err == nil {
				for _, exec := range execs {
					shimMap[exec] = runtimeName
					shimsByRuntime[runtimeName] = appendUnique(shimsByRuntime[runtimeName], exec)
				}
			}

			// On Windows, also check the root version directory for .cmd/.bat files
			// and Scripts directory for Python packages
			if runtime.GOOS == constants.OSWindows {
				if execs, err := findExecutables(versionDir); err == nil {
					for _, exec := range execs {
						shimMap[exec] = runtimeName
						shimsByRuntime[runtimeName] = appendUnique(shimsByRuntime[runtimeName], exec)
					}
				}
				// Check Scripts directory for Python pip packages
				scriptsDir := filepath.Join(versionDir, "Scripts")
				if execs, err := findExecutables(scriptsDir); err == nil {
					for _, exec := range execs {
						shimMap[exec] = runtimeName
						shimsByRuntime[runtimeName] = appendUnique(shimsByRuntime[runtimeName], exec)
					}
				}
			}
		}
	}

	if len(shimMap) == 0 {
		return nil, fmt.Errorf("no runtimes installed - nothing to reshim")
	}

	// Save the shim map cache
	if err := SaveShimMap(shimMap); err != nil {
		return nil, fmt.Errorf("failed to save shim map cache: %w", err)
	}

	// Create all shims
	for shimName := range shimMap {
		if err := m.CreateShim(shimName); err != nil {
			return nil, err
		}
	}

	return &RehashResult{
		ShimsByRuntime: shimsByRuntime,
		TotalShims:     len(shimMap),
	}, nil
}

// Rehash regenerates all shims by scanning installed versions (no progress callback)
func (m *Manager) Rehash() (*RehashResult, error) {
	return m.RehashWithCallback(nil)
}

// appendUnique appends a string to a slice only if it's not already present
func appendUnique(slice []string, s string) []string {
	for _, existing := range slice {
		if existing == s {
			return slice
		}
	}
	return append(slice, s)
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	// Copy contents
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Sync to ensure write is complete
	return dstFile.Sync()
}

// RuntimeShims returns the list of shim names for a given runtime
// For example, Python would return ["python", "python3", "pip", "pip3"]
// This queries the runtime provider for its shims, eliminating the need
// for a central hardcoded mapping.
func RuntimeShims(runtimeName string) []string {
	// Get provider from registry
	provider, err := runtimepkg.Get(runtimeName)
	if err != nil {
		// If provider not found, default to just the runtime name
		return []string{runtimeName}
	}

	// Return the provider's shims
	return provider.Shims()
}

// findExecutables scans a directory for executable files and returns their base names
func findExecutables(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	executables := make([]string, 0)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()

		// On Windows, check for executable extensions
		if runtime.GOOS == constants.OSWindows {
			ext := filepath.Ext(name)
			if ext == ".exe" || ext == ".cmd" || ext == ".bat" {
				// Remove extension for shim name
				baseName := name[:len(name)-len(ext)]
				executables = append(executables, baseName)
			}
		} else {
			// On Unix, check if file has executable bit
			info, err := entry.Info()
			if err != nil {
				continue
			}

			// Check if executable by owner, group, or others
			mode := info.Mode()
			if mode&0111 != 0 {
				executables = append(executables, name)
			}
		}
	}

	return executables, nil
}
