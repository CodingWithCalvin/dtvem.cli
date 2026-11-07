package cmd

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
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

func TestOtherVersions(t *testing.T) {
	installed := func(versions ...string) []runtime.InstalledVersion {
		out := make([]runtime.InstalledVersion, 0, len(versions))
		for _, v := range versions {
			out = append(out, runtime.InstalledVersion{Version: runtime.NewVersion(v)})
		}
		return out
	}

	tests := []struct {
		name      string
		installed []runtime.InstalledVersion
		version   string
		want      []string
	}{
		{
			name:      "Only version installed",
			installed: installed("3.4.7"),
			version:   "3.4.7",
			want:      nil,
		},
		{
			name:      "Other versions installed",
			installed: installed("22.0.0", "22.12.0", "25.2.0"),
			version:   "25.2.0",
			want:      []string{"22.0.0", "22.12.0"},
		},
		{
			name:      "Version not in the list",
			installed: installed("3.12.12", "3.9.9"),
			version:   "3.8.9",
			want:      []string{"3.12.12", "3.9.9"},
		},
		{
			name:      "Nothing installed",
			installed: nil,
			version:   "3.4.7",
			want:      nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := otherVersions(tt.installed, tt.version)
			if !slices.Equal(got, tt.want) {
				t.Errorf("otherVersions(%v, %q) = %v, want %v", tt.installed, tt.version, got, tt.want)
			}
		})
	}
}

func TestLocalPinSource(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DTVEM_ROOT", filepath.Join(tmp, "dtvem"))
	config.ResetPathsCache()
	t.Cleanup(config.ResetPathsCache)

	if err := os.MkdirAll(config.RuntimeVersionPath("ruby", "3.4.7"), 0755); err != nil {
		t.Fatalf("Failed to create installed version: %v", err)
	}
	if err := config.SetGlobalVersion("ruby", "3.4.7"); err != nil {
		t.Fatalf("SetGlobalVersion() setup error: %v", err)
	}

	pinned := filepath.Join(tmp, "pinned")
	unpinned := filepath.Join(tmp, "unpinned")
	for _, dir := range []string{pinned, unpinned} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", dir, err)
		}
	}
	versionFile := filepath.Join(pinned, ".ruby-version")
	if err := os.WriteFile(versionFile, []byte("3.4\n"), 0644); err != nil {
		t.Fatalf("Failed to write .ruby-version: %v", err)
	}

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	chdir := func(dir string) {
		t.Helper()
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("Failed to change directory: %v", err)
		}
	}

	t.Run("Global version is not a local pin", func(t *testing.T) {
		chdir(unpinned)
		if got := localPinSource("ruby", "3.4.7"); got != "" {
			t.Errorf("localPinSource() with only a global version = %q, want \"\"", got)
		}
	})

	t.Run("Project file pins the version", func(t *testing.T) {
		chdir(pinned)
		got := localPinSource("ruby", "3.4.7")
		if got == "" || !sameFile(t, got, versionFile) {
			t.Errorf("localPinSource() = %q, want %q", got, versionFile)
		}
	})

	t.Run("Project file pins a different version", func(t *testing.T) {
		chdir(pinned)
		if got := localPinSource("ruby", "3.3.0"); got != "" {
			t.Errorf("localPinSource() for another version = %q, want \"\"", got)
		}
	})
}

// sameFile reports whether two paths name the same file. Temp paths can
// differ only by symlinks (macOS reports /var/... as /private/var/...).
func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	ai, err := os.Stat(a)
	if err != nil {
		t.Fatalf("Failed to stat %s: %v", a, err)
	}
	bi, err := os.Stat(b)
	if err != nil {
		t.Fatalf("Failed to stat %s: %v", b, err)
	}
	return os.SameFile(ai, bi)
}
