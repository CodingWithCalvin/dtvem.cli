package cmd

import (
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// mockProvider implements runtime.Provider for testing
type mockProvider struct {
	name              string
	displayName       string
	globalVersion     string
	globalSetError    error
	setGlobalCalls    []string
	availableVersions []runtime.AvailableVersion
	listAvailableErr  error
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
	return m.availableVersions, m.listAvailableErr
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

func (m *mockProvider) GetEnvironment(_ string) (map[string]string, error) {
	return map[string]string{}, nil
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

// Helper to create AvailableVersion from a version string
func makeAvailableVersion(v string) runtime.AvailableVersion {
	return runtime.AvailableVersion{
		Version: runtime.NewVersion(v),
	}
}

func TestResolveVersionForProvider_FullVersion(t *testing.T) {
	provider := &mockProvider{
		name:        "node",
		displayName: "Node.js",
		availableVersions: []runtime.AvailableVersion{
			makeAvailableVersion("22.15.0"),
			makeAvailableVersion("22.0.0"),
		},
	}

	// Full version should pass through unchanged
	result, err := resolveVersionForProvider(provider, "22.15.0")
	if err != nil {
		t.Errorf("resolveVersionForProvider returned error: %v", err)
	}
	if result != "22.15.0" {
		t.Errorf("Expected 22.15.0, got %q", result)
	}
}

func TestResolveVersionForProvider_FullVersionWithVPrefix(t *testing.T) {
	provider := &mockProvider{
		name:        "node",
		displayName: "Node.js",
		availableVersions: []runtime.AvailableVersion{
			makeAvailableVersion("22.15.0"),
		},
	}

	// Full version with v prefix should have prefix stripped
	result, err := resolveVersionForProvider(provider, "v22.15.0")
	if err != nil {
		t.Errorf("resolveVersionForProvider returned error: %v", err)
	}
	if result != "22.15.0" {
		t.Errorf("Expected 22.15.0, got %q", result)
	}
}

func TestResolveVersionForProvider_MajorOnly(t *testing.T) {
	provider := &mockProvider{
		name:        "node",
		displayName: "Node.js",
		availableVersions: []runtime.AvailableVersion{
			makeAvailableVersion("22.0.0"),
			makeAvailableVersion("22.5.0"),
			makeAvailableVersion("22.15.0"),
			makeAvailableVersion("22.15.1"),
			makeAvailableVersion("21.0.0"),
		},
	}

	// Major-only should resolve to highest 22.x.x
	result, err := resolveVersionForProvider(provider, "22")
	if err != nil {
		t.Errorf("resolveVersionForProvider returned error: %v", err)
	}
	if result != "22.15.1" {
		t.Errorf("Expected 22.15.1 (highest 22.x.x), got %q", result)
	}
}

func TestResolveVersionForProvider_MajorMinor(t *testing.T) {
	provider := &mockProvider{
		name:        "node",
		displayName: "Node.js",
		availableVersions: []runtime.AvailableVersion{
			makeAvailableVersion("14.21.0"),
			makeAvailableVersion("14.21.3"),
			makeAvailableVersion("14.20.0"),
			makeAvailableVersion("14.20.1"),
		},
	}

	// Major.minor should resolve to highest 14.21.x
	result, err := resolveVersionForProvider(provider, "14.21")
	if err != nil {
		t.Errorf("resolveVersionForProvider returned error: %v", err)
	}
	if result != "14.21.3" {
		t.Errorf("Expected 14.21.3 (highest 14.21.x), got %q", result)
	}
}

func TestResolveVersionForProvider_NoMatch(t *testing.T) {
	provider := &mockProvider{
		name:        "node",
		displayName: "Node.js",
		availableVersions: []runtime.AvailableVersion{
			makeAvailableVersion("22.0.0"),
			makeAvailableVersion("21.0.0"),
		},
	}

	// No matching version should return error
	_, err := resolveVersionForProvider(provider, "99")
	if err == nil {
		t.Error("Expected error for non-matching version, got nil")
	}
}

func TestResolveVersionForProvider_PythonVersions(t *testing.T) {
	provider := &mockProvider{
		name:        "python",
		displayName: "Python",
		availableVersions: []runtime.AvailableVersion{
			makeAvailableVersion("3.9.18"),
			makeAvailableVersion("3.10.13"),
			makeAvailableVersion("3.11.7"),
			makeAvailableVersion("3.12.0"),
			makeAvailableVersion("3.12.1"),
		},
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"3", "3.12.1"},    // Latest 3.x.x
		{"3.11", "3.11.7"}, // Latest 3.11.x
		{"3.12", "3.12.1"}, // Latest 3.12.x
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := resolveVersionForProvider(provider, tt.input)
			if err != nil {
				t.Errorf("resolveVersionForProvider(%q) returned error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("resolveVersionForProvider(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
