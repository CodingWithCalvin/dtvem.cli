package migration

import (
	"testing"
)

// ProviderTestHarness provides a standardized way to test migration provider implementations.
// Each provider package should use this harness to ensure consistent behavior.
type ProviderTestHarness struct {
	Provider     Provider
	ExpectedName string
	Runtime      string
}

// RunAll runs all standard provider tests.
func (h *ProviderTestHarness) RunAll(t *testing.T) {
	t.Run("Name", h.TestName)
	t.Run("DisplayName", h.TestDisplayName)
	t.Run("Runtime", h.TestRuntime)
	t.Run("DetectVersions", h.TestDetectVersions)
	t.Run("CanAutoUninstall", h.TestCanAutoUninstall)
	t.Run("ManualInstructions", h.TestManualInstructions)
}

// TestName verifies the provider returns a valid name.
func (h *ProviderTestHarness) TestName(t *testing.T) {
	name := h.Provider.Name()
	if name == "" {
		t.Error("Name() returned empty string")
	}
	if name != h.ExpectedName {
		t.Errorf("Name() = %q, want %q", name, h.ExpectedName)
	}
}

// TestDisplayName verifies the provider returns a valid display name.
func (h *ProviderTestHarness) TestDisplayName(t *testing.T) {
	displayName := h.Provider.DisplayName()
	if displayName == "" {
		t.Error("DisplayName() returned empty string")
	}
}

// TestRuntime verifies the provider returns the expected runtime.
func (h *ProviderTestHarness) TestRuntime(t *testing.T) {
	runtime := h.Provider.Runtime()
	if runtime != h.Runtime {
		t.Errorf("Runtime() = %q, want %q", runtime, h.Runtime)
	}
}

// TestDetectVersions verifies DetectVersions doesn't error.
func (h *ProviderTestHarness) TestDetectVersions(t *testing.T) {
	versions, err := h.Provider.DetectVersions()
	if err != nil {
		t.Errorf("DetectVersions() error = %v, want nil", err)
	}
	if versions == nil {
		t.Error("DetectVersions() returned nil, want empty slice")
	}
}

// TestCanAutoUninstall verifies the method returns a boolean.
func (h *ProviderTestHarness) TestCanAutoUninstall(t *testing.T) {
	canAuto := h.Provider.CanAutoUninstall()

	// If auto uninstall is supported, UninstallCommand should return non-empty
	if canAuto {
		cmd := h.Provider.UninstallCommand("1.0.0")
		if cmd == "" {
			t.Error("CanAutoUninstall() returns true but UninstallCommand() returns empty")
		}
	}
}

// TestManualInstructions verifies manual instructions are provided.
func (h *ProviderTestHarness) TestManualInstructions(t *testing.T) {
	instructions := h.Provider.ManualInstructions()
	if instructions == "" {
		t.Error("ManualInstructions() returned empty string")
	}
}
