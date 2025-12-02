package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dtvem/dtvem/src/internal/config"
)

func TestUninstallCommand_VersionStripping(t *testing.T) {
	tests := []struct {
		name          string
		inputVersion  string
		expectedClean string
	}{
		{
			name:          "Version with v prefix",
			inputVersion:  "v3.11.0",
			expectedClean: "3.11.0",
		},
		{
			name:          "Version without v prefix",
			inputVersion:  "3.11.0",
			expectedClean: "3.11.0",
		},
		{
			name:          "Version with V prefix (capital)",
			inputVersion:  "V18.0.0",
			expectedClean: "18.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := strings.TrimPrefix(tt.inputVersion, "v")
			result = strings.TrimPrefix(result, "V")

			if result != tt.expectedClean {
				t.Errorf("Version stripping %q = %q, want %q",
					tt.inputVersion, result, tt.expectedClean)
			}
		})
	}
}

func TestUninstallCommand_VersionPathConstruction(t *testing.T) {
	tests := []struct {
		name        string
		runtimeName string
		version     string
	}{
		{
			name:        "Python version path",
			runtimeName: "python",
			version:     "3.11.0",
		},
		{
			name:        "Node version path",
			runtimeName: "node",
			version:     "18.16.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			versionPath := config.RuntimeVersionPath(tt.runtimeName, tt.version)

			// Path should contain runtime name
			if !strings.Contains(versionPath, tt.runtimeName) {
				t.Errorf("Version path %q should contain runtime name %q",
					versionPath, tt.runtimeName)
			}

			// Path should contain version
			if !strings.Contains(versionPath, tt.version) {
				t.Errorf("Version path %q should contain version %q",
					versionPath, tt.version)
			}

			// Path should be absolute
			if !filepath.IsAbs(versionPath) {
				t.Errorf("Version path %q should be absolute", versionPath)
			}
		})
	}
}

func TestUninstallCommand_DirectoryRemoval(t *testing.T) {
	// Create a temporary test directory structure
	tempDir := t.TempDir()
	testVersionDir := filepath.Join(tempDir, "test-runtime", "1.0.0")

	// Create the directory
	err := os.MkdirAll(testVersionDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a test file in the directory
	testFile := filepath.Join(testVersionDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Verify the directory exists
	if _, err := os.Stat(testVersionDir); os.IsNotExist(err) {
		t.Fatalf("Test directory should exist before removal")
	}

	// Remove the directory (simulating uninstall)
	err = os.RemoveAll(testVersionDir)
	if err != nil {
		t.Fatalf("Failed to remove directory: %v", err)
	}

	// Verify the directory no longer exists
	if _, err := os.Stat(testVersionDir); !os.IsNotExist(err) {
		t.Errorf("Test directory should not exist after removal")
	}
}

func TestUninstallCommand_NonExistentVersion(t *testing.T) {
	// Test that checking for a non-existent version works correctly
	tempDir := t.TempDir()
	nonExistentPath := filepath.Join(tempDir, "does-not-exist", "1.0.0")

	// Verify the directory doesn't exist
	_, err := os.Stat(nonExistentPath)
	if !os.IsNotExist(err) {
		t.Errorf("Directory should not exist, but got error: %v", err)
	}

	// This is the check the uninstall command uses
	if _, err := os.Stat(nonExistentPath); os.IsNotExist(err) {
		// Expected: directory doesn't exist
		// The command would show an error message to the user
	} else {
		t.Errorf("Expected IsNotExist error, got: %v", err)
	}
}

func TestUninstallCommand_ConfirmationResponse(t *testing.T) {
	tests := []struct {
		name          string
		response      string
		shouldProceed bool
	}{
		{
			name:          "Lowercase y",
			response:      "y",
			shouldProceed: true,
		},
		{
			name:          "Uppercase Y",
			response:      "Y",
			shouldProceed: true,
		},
		{
			name:          "Lowercase yes",
			response:      "yes",
			shouldProceed: true,
		},
		{
			name:          "Uppercase YES",
			response:      "YES",
			shouldProceed: true,
		},
		{
			name:          "Mixed case Yes",
			response:      "Yes",
			shouldProceed: true,
		},
		{
			name:          "Lowercase n",
			response:      "n",
			shouldProceed: false,
		},
		{
			name:          "Lowercase no",
			response:      "no",
			shouldProceed: false,
		},
		{
			name:          "Empty response",
			response:      "",
			shouldProceed: false,
		},
		{
			name:          "Invalid response",
			response:      "maybe",
			shouldProceed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simulate the confirmation check from the command
			response := strings.ToLower(strings.TrimSpace(tt.response))
			shouldProceed := (response == "y" || response == "yes")

			if shouldProceed != tt.shouldProceed {
				t.Errorf("Response %q: shouldProceed = %v, want %v",
					tt.response, shouldProceed, tt.shouldProceed)
			}
		})
	}
}
