package python

import (
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/testutil"
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

// TestPythonProvider_GetPipURL tests the version-specific pip URL selection
func TestPythonProvider_GetPipURL(t *testing.T) {
	provider := NewProvider()

	tests := []struct {
		name        string
		version     string
		expectedURL string
	}{
		{
			name:        "Python 3.12 uses default URL",
			version:     "3.12.0",
			expectedURL: "https://bootstrap.pypa.io/get-pip.py",
		},
		{
			name:        "Python 3.11 uses default URL",
			version:     "3.11.5",
			expectedURL: "https://bootstrap.pypa.io/get-pip.py",
		},
		{
			name:        "Python 3.10 uses default URL",
			version:     "3.10.0",
			expectedURL: "https://bootstrap.pypa.io/get-pip.py",
		},
		{
			name:        "Python 3.9 uses default URL",
			version:     "3.9.18",
			expectedURL: "https://bootstrap.pypa.io/get-pip.py",
		},
		{
			name:        "Python 3.8 uses version-specific URL",
			version:     "3.8.9",
			expectedURL: "https://bootstrap.pypa.io/pip/3.8/get-pip.py",
		},
		{
			name:        "Python 3.7 uses version-specific URL",
			version:     "3.7.12",
			expectedURL: "https://bootstrap.pypa.io/pip/3.7/get-pip.py",
		},
		{
			name:        "Python 3.6 uses version-specific URL",
			version:     "3.6.15",
			expectedURL: "https://bootstrap.pypa.io/pip/3.6/get-pip.py",
		},
		{
			name:        "Python 2.7 uses version-specific URL",
			version:     "2.7.18",
			expectedURL: "https://bootstrap.pypa.io/get-pip.py", // 2.x uses default (not 3.x)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := provider.getPipURL(tt.version)
			if url != tt.expectedURL {
				t.Errorf("getPipURL(%q) = %q, want %q", tt.version, url, tt.expectedURL)
			}
		})
	}
}
