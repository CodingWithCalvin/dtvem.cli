package python

import (
	"testing"

	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/testutil"
)

// TestPythonProviderContract runs the generic provider test harness
// This ensures the Python provider correctly implements the Provider interface
func TestPythonProviderContract(t *testing.T) {
	provider := NewProvider()

	harness := &runtime.ProviderTestHarness{
		Provider:            provider,
		T:                   t,
		ExpectedName:        "python",
		ExpectedDisplayName: "Python",
		SampleVersion:       "3.11.0", // Stable version
	}

	harness.RunAllTests()
}

// TestPythonProvider_SpecificBehavior tests Python-specific functionality
func TestPythonProvider_SpecificBehavior(t *testing.T) {
	provider := NewProvider()

	t.Run("Name is lowercase", func(t *testing.T) {
		if provider.Name() != "python" {
			t.Errorf("Name() = %q, want \"python\"", provider.Name())
		}
	})

	t.Run("DisplayName is Python", func(t *testing.T) {
		displayName := provider.DisplayName()
		if displayName != "Python" {
			t.Errorf("DisplayName() = %q, want \"Python\"", displayName)
		}
	})

	t.Run("GetManualPackageInstallCommand uses pip", func(t *testing.T) {
		cmd := provider.ManualPackageInstallCommand([]string{"requests", "flask"})
		if cmd == "" {
			t.Fatal("GetManualPackageInstallCommand() returned empty string")
		}

		// Should use pip install (not -g like npm)
		if cmd != "pip install requests flask" {
			t.Errorf("GetManualPackageInstallCommand() = %q, expected pip install format", cmd)
		}
	})

	t.Run("GetManualPackageInstallCommand empty packages", func(t *testing.T) {
		cmd := provider.ManualPackageInstallCommand([]string{})
		if cmd != "" {
			t.Errorf("GetManualPackageInstallCommand([]) = %q, want empty string", cmd)
		}
	})
}

// TestPythonProvider_InstallPath tests install path structure
func TestPythonProvider_InstallPath(t *testing.T) {
	provider := NewProvider()

	version := "3.11.0"
	path, err := provider.InstallPath(version)

	// May error if not installed, but if it returns a path, validate format
	if err == nil {
		if path == "" {
			t.Error("GetInstallPath() returned empty path without error")
		}

		// Should contain "python" and the version
		if !testutil.ContainsSubstring(path, "python") {
			t.Errorf("GetInstallPath() = %q does not contain 'python'", path)
		}
		if !testutil.ContainsSubstring(path, version) {
			t.Errorf("GetInstallPath() = %q does not contain version %q", path, version)
		}
	}
}
