package runtime

import (
	"strings"
	"testing"
)

// ProviderTestHarness runs a suite of contract tests against a Provider implementation
// This ensures all providers behave consistently and implement the interface correctly
type ProviderTestHarness struct {
	Provider Provider
	T        *testing.T

	// Expected values for validation
	ExpectedName        string
	ExpectedDisplayName string
	SampleVersion       string // A valid version string for this runtime (e.g., "3.11.0")
}

// RunAllTests executes the complete test suite
func (h *ProviderTestHarness) RunAllTests() {
	h.T.Run("Name", func(t *testing.T) { h.TestName(t) })
	h.T.Run("DisplayName", func(t *testing.T) { h.TestDisplayName(t) })
	h.T.Run("GetInstallPath", func(t *testing.T) { h.TestGetInstallPath(t) })
	h.T.Run("GetExecutablePath", func(t *testing.T) { h.TestGetExecutablePath(t) })
	h.T.Run("GetGlobalPackages", func(t *testing.T) { h.TestGetGlobalPackages(t) })
	h.T.Run("GetManualPackageInstallCommand", func(t *testing.T) { h.TestGetManualPackageInstallCommand(t) })
	h.T.Run("ListInstalled", func(t *testing.T) { h.TestListInstalled(t) })
	h.T.Run("ListAvailable", func(t *testing.T) { h.TestListAvailable(t) })
	h.T.Run("IsInstalled", func(t *testing.T) { h.TestIsInstalled(t) })
	h.T.Run("GetGlobalVersion", func(t *testing.T) { h.TestGetGlobalVersion(t) })
	h.T.Run("GetLocalVersion", func(t *testing.T) { h.TestGetLocalVersion(t) })
	h.T.Run("GetCurrentVersion", func(t *testing.T) { h.TestGetCurrentVersion(t) })
}

// TestName verifies the provider returns the expected name
func (h *ProviderTestHarness) TestName(t *testing.T) {
	name := h.Provider.Name()

	if name == "" {
		t.Error("Name() returned empty string")
	}

	if name != h.ExpectedName {
		t.Errorf("Name() = %q, want %q", name, h.ExpectedName)
	}

	// Name should be lowercase (convention)
	if name != strings.ToLower(name) {
		t.Errorf("Name() = %q should be lowercase", name)
	}
}

// TestDisplayName verifies the provider returns a human-readable name
func (h *ProviderTestHarness) TestDisplayName(t *testing.T) {
	displayName := h.Provider.DisplayName()

	if displayName == "" {
		t.Error("DisplayName() returned empty string")
	}

	if displayName != h.ExpectedDisplayName {
		t.Errorf("DisplayName() = %q, want %q", displayName, h.ExpectedDisplayName)
	}
}

// TestGetInstallPath verifies install path follows expected patterns
func (h *ProviderTestHarness) TestGetInstallPath(t *testing.T) {
	if h.SampleVersion == "" {
		t.Skip("No sample version provided")
	}

	path, err := h.Provider.InstallPath(h.SampleVersion)

	// It's OK to return error if version isn't installed
	// But the path format should still be correct if returned
	if err == nil {
		if path == "" {
			t.Error("GetInstallPath() returned empty path without error")
		}

		// Path should contain the runtime name
		if !strings.Contains(strings.ToLower(path), h.Provider.Name()) {
			t.Errorf("GetInstallPath() = %q does not contain runtime name %q", path, h.Provider.Name())
		}

		// Path should contain the version
		if !strings.Contains(path, h.SampleVersion) {
			t.Errorf("GetInstallPath() = %q does not contain version %q", path, h.SampleVersion)
		}
	}
}

// TestGetExecutablePath verifies executable path follows expected patterns
func (h *ProviderTestHarness) TestGetExecutablePath(t *testing.T) {
	if h.SampleVersion == "" {
		t.Skip("No sample version provided")
	}

	path, err := h.Provider.ExecutablePath(h.SampleVersion)

	// It's OK to return error if version isn't installed
	if err == nil {
		if path == "" {
			t.Error("GetExecutablePath() returned empty path without error")
		}

		// Path should be non-empty and look like a file path
		if !strings.Contains(path, "/") && !strings.Contains(path, "\\") {
			t.Errorf("GetExecutablePath() = %q does not look like a valid path", path)
		}
	}
}

// TestGetGlobalPackages verifies package detection doesn't panic and returns valid data
func (h *ProviderTestHarness) TestGetGlobalPackages(t *testing.T) {
	// Test with a fake install path
	packages, err := h.Provider.GlobalPackages("/fake/path")

	// Should not panic and should return a slice (even if empty or error)
	if err == nil && packages == nil {
		t.Error("GetGlobalPackages() returned nil slice without error (should return empty slice)")
	}

	// If no error, should return valid slice
	if err == nil {
		for i, pkg := range packages {
			if pkg == "" {
				t.Errorf("GetGlobalPackages()[%d] is empty string", i)
			}
		}
	}
}

// TestGetManualPackageInstallCommand verifies manual install commands are properly formatted
func (h *ProviderTestHarness) TestGetManualPackageInstallCommand(t *testing.T) {
	tests := []struct {
		name     string
		packages []string
		wantNil  bool
	}{
		{
			name:     "empty package list",
			packages: []string{},
			wantNil:  true, // Empty input should return empty string
		},
		{
			name:     "single package",
			packages: []string{"test-package"},
			wantNil:  false,
		},
		{
			name:     "multiple packages",
			packages: []string{"package1", "package2", "package3"},
			wantNil:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := h.Provider.ManualPackageInstallCommand(tt.packages)

			if tt.wantNil && cmd != "" {
				t.Errorf("GetManualPackageInstallCommand(%v) = %q, want empty string", tt.packages, cmd)
			}

			if !tt.wantNil && cmd == "" {
				t.Errorf("GetManualPackageInstallCommand(%v) returned empty string", tt.packages)
			}

			// If command is returned, verify it contains the packages
			if cmd != "" {
				for _, pkg := range tt.packages {
					if !strings.Contains(cmd, pkg) {
						t.Errorf("GetManualPackageInstallCommand() = %q does not contain package %q", cmd, pkg)
					}
				}
			}
		})
	}
}

// TestListInstalled verifies listing installed versions returns valid data
func (h *ProviderTestHarness) TestListInstalled(t *testing.T) {
	versions, err := h.Provider.ListInstalled()

	// Should not panic
	// It's OK to have error or no versions if none are installed
	if err == nil && versions == nil {
		t.Error("ListInstalled() returned nil slice without error (should return empty slice)")
	}

	// If versions returned, validate structure
	if err == nil {
		for i, version := range versions {
			if version.Version.Raw == "" {
				t.Errorf("ListInstalled()[%d].Version.Raw is empty", i)
			}
			if version.InstallPath == "" {
				t.Errorf("ListInstalled()[%d].InstallPath is empty", i)
			}
		}
	}
}

// TestListAvailable verifies listing available versions returns valid data
func (h *ProviderTestHarness) TestListAvailable(t *testing.T) {
	// Note: This may require network access, so we just verify it doesn't panic
	// and returns proper structure
	versions, err := h.Provider.ListAvailable()

	// Should not panic
	if err == nil && versions == nil {
		t.Error("ListAvailable() returned nil slice without error (should return empty slice)")
	}

	// If versions returned, validate structure
	if err == nil && len(versions) > 0 {
		for i, version := range versions {
			if version.Version.Raw == "" {
				t.Errorf("ListAvailable()[%d].Version.Raw is empty", i)
			}
			// DownloadURL might be empty for some versions, so we don't check it
		}
	}
}

// TestIsInstalled verifies version checking works correctly
func (h *ProviderTestHarness) TestIsInstalled(t *testing.T) {
	if h.SampleVersion == "" {
		t.Skip("No sample version provided")
	}

	// Check a version (may or may not be installed)
	installed, err := h.Provider.IsInstalled(h.SampleVersion)

	// Should return boolean without panic
	// Error is acceptable
	_ = installed
	_ = err

	// Test with obviously invalid version
	invalidInstalled, invalidErr := h.Provider.IsInstalled("999.999.999")
	if invalidErr == nil && invalidInstalled {
		t.Error("IsInstalled(\"999.999.999\") returned true (should be false)")
	}
}

// TestGetGlobalVersion verifies global version retrieval
func (h *ProviderTestHarness) TestGetGlobalVersion(t *testing.T) {
	version, err := h.Provider.GlobalVersion()

	// ("", nil) is valid — means no global version configured
	// If a version is returned, it should be non-empty
	if err == nil && version != "" {
		// Valid: a global version is configured
	}
	if err != nil {
		// Valid: an actual error occurred (I/O, corrupt config)
		t.Logf("GetGlobalVersion() returned error (may be expected): %v", err)
	}
}

// TestGetLocalVersion verifies local version retrieval
func (h *ProviderTestHarness) TestGetLocalVersion(t *testing.T) {
	version, err := h.Provider.LocalVersion()

	// It's OK to have error if no local version is set
	// But if version is returned, it should be non-empty
	if err == nil && version == "" {
		t.Error("GetLocalVersion() returned empty string without error")
	}
}

// TestGetCurrentVersion verifies current version resolution
func (h *ProviderTestHarness) TestGetCurrentVersion(t *testing.T) {
	version, err := h.Provider.CurrentVersion()

	// It's OK to have error if no version is configured
	// But if version is returned, it should be non-empty
	if err == nil && version == "" {
		t.Error("GetCurrentVersion() returned empty string without error")
	}
}

// TestInstallGlobalPackages verifies package installation interface
func (h *ProviderTestHarness) TestInstallGlobalPackages(t *testing.T) {
	if h.SampleVersion == "" {
		t.Skip("No sample version provided")
	}

	// Test with empty package list (should not error)
	err := h.Provider.InstallGlobalPackages(h.SampleVersion, []string{})
	if err != nil {
		t.Errorf("InstallGlobalPackages() with empty packages returned error: %v", err)
	}

	// Note: We don't test with actual packages as that would require
	// the version to be installed and would modify the system
}
