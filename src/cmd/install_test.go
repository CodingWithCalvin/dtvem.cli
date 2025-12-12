package cmd

import (
	"testing"

	"github.com/dtvem/dtvem/src/internal/runtime"
)

// mockProvider implements runtime.Provider for testing
type mockProvider struct {
	name           string
	displayName    string
	globalVersion  string
	globalSetError error
	setGlobalCalls []string
}

func (m *mockProvider) Name() string                                          { return m.name }
func (m *mockProvider) DisplayName() string                                   { return m.displayName }
func (m *mockProvider) Shims() []string                                       { return []string{m.name} }
func (m *mockProvider) ExecutablePath(version string) (string, error)         { return "", nil }
func (m *mockProvider) IsInstalled(version string) (bool, error)              { return false, nil }
func (m *mockProvider) ShouldReshimAfter(shimName string, args []string) bool { return false }
func (m *mockProvider) Install(version string) error                          { return nil }
func (m *mockProvider) Uninstall(version string) error                        { return nil }
func (m *mockProvider) ListInstalled() ([]runtime.InstalledVersion, error) {
	return nil, nil
}
func (m *mockProvider) ListAvailable() ([]runtime.AvailableVersion, error) {
	return nil, nil
}
func (m *mockProvider) InstallPath(version string) (string, error) { return "", nil }
func (m *mockProvider) LocalVersion() (string, error)              { return "", nil }
func (m *mockProvider) SetLocalVersion(version string) error       { return nil }
func (m *mockProvider) CurrentVersion() (string, error)            { return "", nil }
func (m *mockProvider) DetectInstalled() ([]runtime.DetectedVersion, error) {
	return nil, nil
}
func (m *mockProvider) GlobalPackages(installPath string) ([]string, error) {
	return nil, nil
}
func (m *mockProvider) InstallGlobalPackages(version string, packages []string) error {
	return nil
}
func (m *mockProvider) ManualPackageInstallCommand(packages []string) string {
	return ""
}

func (m *mockProvider) GlobalVersion() (string, error) {
	return m.globalVersion, nil
}

func (m *mockProvider) SetGlobalVersion(version string) error {
	m.setGlobalCalls = append(m.setGlobalCalls, version)
	return m.globalSetError
}

func TestAutoSetGlobalIfNeeded_NoGlobalVersion(t *testing.T) {
	provider := &mockProvider{
		name:          "test",
		displayName:   "Test",
		globalVersion: "", // No global version set
	}

	autoSetGlobalIfNeeded(provider, "1.0.0")

	if len(provider.setGlobalCalls) != 1 {
		t.Errorf("Expected SetGlobalVersion to be called once, got %d calls", len(provider.setGlobalCalls))
	}
	if len(provider.setGlobalCalls) > 0 && provider.setGlobalCalls[0] != "1.0.0" {
		t.Errorf("Expected SetGlobalVersion called with '1.0.0', got %q", provider.setGlobalCalls[0])
	}
}

func TestAutoSetGlobalIfNeeded_GlobalVersionAlreadySet(t *testing.T) {
	provider := &mockProvider{
		name:          "test",
		displayName:   "Test",
		globalVersion: "2.0.0", // Global version already set
	}

	autoSetGlobalIfNeeded(provider, "1.0.0")

	if len(provider.setGlobalCalls) != 0 {
		t.Errorf("Expected SetGlobalVersion to not be called when global already set, got %d calls", len(provider.setGlobalCalls))
	}
}

func TestAutoSetGlobalIfNeeded_MultipleInstalls(t *testing.T) {
	provider := &mockProvider{
		name:          "test",
		displayName:   "Test",
		globalVersion: "", // No global version initially
	}

	// First install - should set global
	autoSetGlobalIfNeeded(provider, "1.0.0")

	if len(provider.setGlobalCalls) != 1 {
		t.Fatalf("Expected first install to set global, got %d calls", len(provider.setGlobalCalls))
	}

	// Simulate that global is now set
	provider.globalVersion = "1.0.0"

	// Second install - should NOT change global
	autoSetGlobalIfNeeded(provider, "2.0.0")

	if len(provider.setGlobalCalls) != 1 {
		t.Errorf("Expected second install to not change global, got %d calls total", len(provider.setGlobalCalls))
	}
}
