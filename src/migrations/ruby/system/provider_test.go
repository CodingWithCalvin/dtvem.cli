package system

import (
	"testing"

	"github.com/dtvem/dtvem/src/internal/migration"
)

func TestProvider(t *testing.T) {
	harness := &migration.ProviderTestHarness{
		Provider:     NewProvider(),
		ExpectedName: "system-ruby",
		Runtime:      "ruby",
	}
	harness.RunAll(t)
}

func TestProvider_CanAutoUninstall(t *testing.T) {
	p := NewProvider()

	if p.CanAutoUninstall() {
		t.Error("CanAutoUninstall() = true, want false")
	}
}

func TestProvider_UninstallCommand(t *testing.T) {
	p := NewProvider()

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
}
