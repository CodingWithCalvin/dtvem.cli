package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dtvem/dtvem/src/internal/config"
)

func TestVersionValidation_InstalledVersion(t *testing.T) {
	// Create a temporary dtvem root directory
	tempDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	if err := os.Setenv("DTVEM_ROOT", tempDir); err != nil {
		t.Fatalf("Failed to set DTVEM_ROOT: %v", err)
	}
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	// Reset cached paths to use the new root
	config.ResetPathsCache()
	defer config.ResetPathsCache()

	// Create a fake installed version directory for python
	versionDir := filepath.Join(tempDir, "versions", "python", "3.11.0")
	err := os.MkdirAll(versionDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create version directory: %v", err)
	}

	// Verify the directory exists (simulating an installed version)
	if _, err := os.Stat(versionDir); os.IsNotExist(err) {
		t.Fatalf("Version directory should exist")
	}
}

func TestVersionValidation_NotInstalledVersion(t *testing.T) {
	// Create a temporary dtvem root directory
	tempDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	if err := os.Setenv("DTVEM_ROOT", tempDir); err != nil {
		t.Fatalf("Failed to set DTVEM_ROOT: %v", err)
	}
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	// Reset cached paths to use the new root
	config.ResetPathsCache()
	defer config.ResetPathsCache()

	// Version directory does not exist (not installed)
	versionDir := filepath.Join(tempDir, "versions", "python", "3.99.0")

	// Verify the directory does NOT exist
	if _, err := os.Stat(versionDir); !os.IsNotExist(err) {
		t.Fatalf("Version directory should not exist for uninstalled version")
	}
}

func TestVersionValidation_VersionPathFormat(t *testing.T) {
	tests := []struct {
		name        string
		runtimeName string
		version     string
	}{
		{
			name:        "Python version",
			runtimeName: "python",
			version:     "3.11.0",
		},
		{
			name:        "Node version",
			runtimeName: "node",
			version:     "18.16.0",
		},
		{
			name:        "Version with only major.minor",
			runtimeName: "python",
			version:     "3.12",
		},
		{
			name:        "Typo version (issue example)",
			runtimeName: "python",
			version:     "3.47",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionPath := config.RuntimeVersionPath(tt.runtimeName, tt.version)

			// Path should be absolute
			if !filepath.IsAbs(versionPath) {
				t.Errorf("Version path should be absolute, got: %s", versionPath)
			}

			// Path should contain the runtime name
			if !containsPathComponent(versionPath, tt.runtimeName) {
				t.Errorf("Version path should contain runtime name %q, got: %s", tt.runtimeName, versionPath)
			}

			// Path should contain the version
			if !containsPathComponent(versionPath, tt.version) {
				t.Errorf("Version path should contain version %q, got: %s", tt.version, versionPath)
			}
		})
	}
}

// containsPathComponent checks if a path contains a specific component
func containsPathComponent(path, component string) bool {
	// Split path and check each component
	for _, part := range filepath.SplitList(path) {
		if part == component {
			return true
		}
	}
	// Also check using string contains as fallback for nested paths
	return filepath.Base(path) == component ||
		filepath.Base(filepath.Dir(path)) == component ||
		filepath.Base(filepath.Dir(filepath.Dir(path))) == component
}
