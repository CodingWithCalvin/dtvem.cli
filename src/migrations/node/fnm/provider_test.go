package fnm

import (
	"testing"

	"github.com/dtvem/dtvem/src/internal/migration"
)

func TestProvider(t *testing.T) {
	harness := &migration.ProviderTestHarness{
		Provider:     NewProvider(),
		ExpectedName: "fnm",
		Runtime:      "node",
	}
	harness.RunAll(t)
}

func TestProvider_UninstallCommand(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		version  string
		expected string
	}{
		{version: "22.0.0", expected: "fnm uninstall 22.0.0"},
		{version: "18.16.0", expected: "fnm uninstall 18.16.0"},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := p.UninstallCommand(tt.version)
			if result != tt.expected {
				t.Errorf("UninstallCommand(%q) = %q, want %q", tt.version, result, tt.expected)
			}
		})
	}
}
