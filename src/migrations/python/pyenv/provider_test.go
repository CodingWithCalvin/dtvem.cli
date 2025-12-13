package pyenv

import (
	"testing"

	"github.com/dtvem/dtvem/src/internal/migration"
)

func TestProvider(t *testing.T) {
	harness := &migration.ProviderTestHarness{
		Provider:     NewProvider(),
		ExpectedName: "pyenv",
		Runtime:      "python",
	}
	harness.RunAll(t)
}

func TestProvider_UninstallCommand(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		version  string
		expected string
	}{
		{version: "3.11.0", expected: "pyenv uninstall 3.11.0"},
		{version: "3.12.0", expected: "pyenv uninstall 3.12.0"},
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
