package chruby

import (
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/migration"
)

func TestProvider(t *testing.T) {
	harness := &migration.ProviderTestHarness{
		Provider:     NewProvider(),
		ExpectedName: "chruby",
		Runtime:      "ruby",
	}
	harness.RunAll(t)
}

func TestProvider_CanAutoUninstall(t *testing.T) {
	p := NewProvider()

	// chruby doesn't support automatic uninstall
	if p.CanAutoUninstall() {
		t.Error("CanAutoUninstall() = true, want false")
	}
}

func TestProvider_UninstallCommand(t *testing.T) {
	p := NewProvider()

	// chruby doesn't have an uninstall command
	if cmd := p.UninstallCommand("3.3.0"); cmd != "" {
		t.Errorf("UninstallCommand() = %q, want empty string", cmd)
	}
}

func TestProvider_ManualInstructions(t *testing.T) {
	p := NewProvider()

	instructions := p.ManualInstructions()
	if instructions == "" {
		t.Error("ManualInstructions() returned empty string")
	}

	// Check that instructions mention removing directories
	if !containsSubstring(instructions, "rubies") {
		t.Error("ManualInstructions() should mention rubies directory")
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
