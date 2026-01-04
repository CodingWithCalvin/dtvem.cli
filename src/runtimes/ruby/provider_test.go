package ruby

import (
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/testutil"
)

// TestRubyProviderContract runs the generic provider test harness
// This ensures the Ruby provider correctly implements the Provider interface
func TestRubyProviderContract(t *testing.T) {
	provider := NewProvider()

	harness := &runtime.ProviderTestHarness{
		Provider:            provider,
		T:                   t,
		ExpectedName:        "ruby",
		ExpectedDisplayName: "Ruby",
		SampleVersion:       "3.3.0", // Recent stable version
	}

	harness.RunAllTests()
}

// TestRubyProvider_SpecificBehavior tests Ruby-specific functionality
func TestRubyProvider_SpecificBehavior(t *testing.T) {
	provider := NewProvider()

	t.Run("Name is lowercase", func(t *testing.T) {
		if provider.Name() != "ruby" {
			t.Errorf("Name() = %q, want \"ruby\"", provider.Name())
		}
	})

	t.Run("DisplayName is Ruby", func(t *testing.T) {
		displayName := provider.DisplayName()
		if displayName != "Ruby" {
			t.Errorf("DisplayName() = %q, want \"Ruby\"", displayName)
		}
	})

	t.Run("Shims includes expected executables", func(t *testing.T) {
		shims := provider.Shims()
		expectedShims := []string{"ruby", "gem", "irb", "bundle", "rake", "rdoc", "ri"}

		if len(shims) != len(expectedShims) {
			t.Errorf("Shims() returned %d shims, want %d", len(shims), len(expectedShims))
		}

		for _, expected := range expectedShims {
			found := false
			for _, shim := range shims {
				if shim == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Shims() does not include %q", expected)
			}
		}
	})

	t.Run("ManualPackageInstallCommand uses gem", func(t *testing.T) {
		cmd := provider.ManualPackageInstallCommand([]string{"rails", "sinatra"})
		if cmd == "" {
			t.Fatal("ManualPackageInstallCommand() returned empty string")
		}

		// Should use gem install
		if cmd != "gem install rails sinatra" {
			t.Errorf("ManualPackageInstallCommand() = %q, expected gem install format", cmd)
		}
	})

	t.Run("ManualPackageInstallCommand empty packages", func(t *testing.T) {
		cmd := provider.ManualPackageInstallCommand([]string{})
		if cmd != "" {
			t.Errorf("ManualPackageInstallCommand([]) = %q, want empty string", cmd)
		}
	})
}

// TestRubyProvider_InstallPath tests install path structure
func TestRubyProvider_InstallPath(t *testing.T) {
	provider := NewProvider()

	version := "3.2.0"
	path, err := provider.InstallPath(version)

	// May error if not installed, but if it returns a path, validate format
	if err == nil {
		if path == "" {
			t.Error("InstallPath() returned empty path without error")
		}

		// Should contain "ruby" and the version
		if !testutil.ContainsSubstring(path, "ruby") {
			t.Errorf("InstallPath() = %q does not contain 'ruby'", path)
		}
		if !testutil.ContainsSubstring(path, version) {
			t.Errorf("InstallPath() = %q does not contain version %q", path, version)
		}
	}
}

// TestRubyProvider_ShouldReshimAfter tests reshim detection
func TestRubyProvider_ShouldReshimAfter(t *testing.T) {
	provider := NewProvider()

	tests := []struct {
		name     string
		shimName string
		args     []string
		want     bool
	}{
		{
			name:     "gem install should reshim",
			shimName: "gem",
			args:     []string{"install", "rails"},
			want:     true,
		},
		{
			name:     "gem uninstall should reshim",
			shimName: "gem",
			args:     []string{"uninstall", "rails"},
			want:     true,
		},
		{
			name:     "gem list should not reshim",
			shimName: "gem",
			args:     []string{"list"},
			want:     false,
		},
		{
			name:     "bundle install should reshim",
			shimName: "bundle",
			args:     []string{"install"},
			want:     true,
		},
		{
			name:     "bundle update should reshim",
			shimName: "bundle",
			args:     []string{"update"},
			want:     true,
		},
		{
			name:     "bundle exec should not reshim",
			shimName: "bundle",
			args:     []string{"exec", "rails", "server"},
			want:     false,
		},
		{
			name:     "ruby should not reshim",
			shimName: "ruby",
			args:     []string{"script.rb"},
			want:     false,
		},
		{
			name:     "irb should not reshim",
			shimName: "irb",
			args:     []string{},
			want:     false,
		},
		{
			name:     "empty args should not reshim",
			shimName: "gem",
			args:     []string{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provider.ShouldReshimAfter(tt.shimName, tt.args)
			if got != tt.want {
				t.Errorf("ShouldReshimAfter(%q, %v) = %v, want %v",
					tt.shimName, tt.args, got, tt.want)
			}
		})
	}
}
