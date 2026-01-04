package node

import (
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/testutil"
)

// TestNodeProviderContract runs the generic provider test harness
// This ensures the Node.js provider correctly implements the Provider interface
func TestNodeProviderContract(t *testing.T) {
	provider := NewProvider()

	harness := &runtime.ProviderTestHarness{
		Provider:            provider,
		T:                   t,
		ExpectedName:        "node",
		ExpectedDisplayName: "Node.js",
		SampleVersion:       "20.11.0", // Recent LTS version
	}

	harness.RunAllTests()
}

// TestNodeProvider_SpecificBehavior tests Node.js-specific functionality
func TestNodeProvider_SpecificBehavior(t *testing.T) {
	provider := NewProvider()

	t.Run("Name is lowercase", func(t *testing.T) {
		if provider.Name() != "node" {
			t.Errorf("Name() = %q, want \"node\"", provider.Name())
		}
	})

	t.Run("DisplayName is Node.js", func(t *testing.T) {
		displayName := provider.DisplayName()
		if displayName != "Node.js" {
			t.Errorf("DisplayName() = %q, want \"Node.js\"", displayName)
		}
	})

	t.Run("GetManualPackageInstallCommand uses npm", func(t *testing.T) {
		cmd := provider.ManualPackageInstallCommand([]string{"express", "lodash"})
		if cmd == "" {
			t.Fatal("GetManualPackageInstallCommand() returned empty string")
		}

		// Should use npm install -g
		if cmd != "npm install -g express lodash" {
			t.Errorf("GetManualPackageInstallCommand() = %q, expected npm install -g format", cmd)
		}
	})

	t.Run("GetManualPackageInstallCommand empty packages", func(t *testing.T) {
		cmd := provider.ManualPackageInstallCommand([]string{})
		if cmd != "" {
			t.Errorf("GetManualPackageInstallCommand([]) = %q, want empty string", cmd)
		}
	})
}

// TestNodeProvider_InstallPath tests install path structure
func TestNodeProvider_InstallPath(t *testing.T) {
	provider := NewProvider()

	version := "18.16.0"
	path, err := provider.InstallPath(version)

	// May error if not installed, but if it returns a path, validate format
	if err == nil {
		if path == "" {
			t.Error("GetInstallPath() returned empty path without error")
		}

		// Should contain "node" and the version
		if !testutil.ContainsSubstring(path, "node") {
			t.Errorf("GetInstallPath() = %q does not contain 'node'", path)
		}
		if !testutil.ContainsSubstring(path, version) {
			t.Errorf("GetInstallPath() = %q does not contain version %q", path, version)
		}
	}
}
