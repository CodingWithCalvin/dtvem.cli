package python

import (
	"os"
	"path/filepath"
	"strings"
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

// TestPythonProvider_EnableSitePackages tests the ._pth file modification
func TestPythonProvider_EnableSitePackages(t *testing.T) {
	provider := NewProvider()

	t.Run("returns error for non-existent file", func(t *testing.T) {
		err := provider.enableSitePackages("/nonexistent/path/python311._pth")
		if err == nil {
			t.Error("enableSitePackages() should return error for non-existent file")
		}
	})

	t.Run("uncomments import site line", func(t *testing.T) {
		// Create a temp file with commented import site
		tempDir := t.TempDir()
		pthFile := filepath.Join(tempDir, "python311._pth")
		content := "python311.zip\n.\n#import site\n"
		if err := os.WriteFile(pthFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := provider.enableSitePackages(pthFile)
		if err != nil {
			t.Fatalf("enableSitePackages() returned error: %v", err)
		}

		// Read and verify
		result, err := os.ReadFile(pthFile)
		if err != nil {
			t.Fatalf("Failed to read result file: %v", err)
		}

		if !strings.Contains(string(result), "import site") {
			t.Error("Result should contain 'import site'")
		}
		if strings.Contains(string(result), "#import site") {
			t.Error("Result should not contain commented '#import site'")
		}
	})

	t.Run("adds import site if missing", func(t *testing.T) {
		// Create a temp file without import site
		tempDir := t.TempDir()
		pthFile := filepath.Join(tempDir, "python311._pth")
		content := "python311.zip\n.\n"
		if err := os.WriteFile(pthFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := provider.enableSitePackages(pthFile)
		if err != nil {
			t.Fatalf("enableSitePackages() returned error: %v", err)
		}

		// Read and verify
		result, err := os.ReadFile(pthFile)
		if err != nil {
			t.Fatalf("Failed to read result file: %v", err)
		}

		if !strings.Contains(string(result), "import site") {
			t.Error("Result should contain 'import site'")
		}
	})

	t.Run("preserves already uncommented import site", func(t *testing.T) {
		// Create a temp file with already uncommented import site
		tempDir := t.TempDir()
		pthFile := filepath.Join(tempDir, "python311._pth")
		content := "python311.zip\n.\nimport site\n"
		if err := os.WriteFile(pthFile, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := provider.enableSitePackages(pthFile)
		if err != nil {
			t.Fatalf("enableSitePackages() returned error: %v", err)
		}

		// Read and verify - should still have import site, not duplicated
		result, err := os.ReadFile(pthFile)
		if err != nil {
			t.Fatalf("Failed to read result file: %v", err)
		}

		count := strings.Count(string(result), "import site")
		if count != 1 {
			t.Errorf("Expected exactly 1 'import site' line, got %d", count)
		}
	})
}

// TestPythonProvider_Shims tests the shim configuration
func TestPythonProvider_Shims(t *testing.T) {
	provider := NewProvider()

	shims := provider.Shims()

	t.Run("includes python shim", func(t *testing.T) {
		found := false
		for _, s := range shims {
			if s == "python" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Shims() should include 'python'")
		}
	})

	t.Run("includes pip shim", func(t *testing.T) {
		found := false
		for _, s := range shims {
			if s == "pip" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Shims() should include 'pip'")
		}
	})

	t.Run("includes pip3 shim", func(t *testing.T) {
		found := false
		for _, s := range shims {
			if s == "pip3" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Shims() should include 'pip3'")
		}
	})
}
