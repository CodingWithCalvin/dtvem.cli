package nvm

import (
	"testing"
)

func TestProvider_Name(t *testing.T) {
	p := NewProvider()
	if name := p.Name(); name != "nvm" {
		t.Errorf("Name() = %q, want %q", name, "nvm")
	}
}

func TestProvider_DisplayName(t *testing.T) {
	p := NewProvider()
	expected := "Node Version Manager (nvm)"
	if name := p.DisplayName(); name != expected {
		t.Errorf("DisplayName() = %q, want %q", name, expected)
	}
}

func TestProvider_Runtime(t *testing.T) {
	p := NewProvider()
	if runtime := p.Runtime(); runtime != "node" {
		t.Errorf("Runtime() = %q, want %q", runtime, "node")
	}
}

func TestProvider_CanAutoUninstall(t *testing.T) {
	p := NewProvider()
	if !p.CanAutoUninstall() {
		t.Error("CanAutoUninstall() = false, want true")
	}
}

func TestProvider_UninstallCommand(t *testing.T) {
	p := NewProvider()

	tests := []struct {
		version  string
		expected string
	}{
		{version: "22.0.0", expected: "nvm uninstall 22.0.0"},
		{version: "18.16.0", expected: "nvm uninstall 18.16.0"},
		{version: "20.10.0", expected: "nvm uninstall 20.10.0"},
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

func TestProvider_ManualInstructions(t *testing.T) {
	p := NewProvider()
	instructions := p.ManualInstructions()

	if instructions == "" {
		t.Error("ManualInstructions() returned empty string")
	}

	// Check that instructions mention nvm
	if !containsSubstring(instructions, "nvm") {
		t.Error("ManualInstructions() should mention nvm")
	}
}

func TestProvider_DetectVersions_NoInstallation(t *testing.T) {
	p := NewProvider()

	// DetectVersions should not error even if nvm is not installed
	versions, err := p.DetectVersions()
	if err != nil {
		t.Errorf("DetectVersions() error = %v, want nil", err)
	}

	// The result may be empty if nvm is not installed, which is fine
	if versions == nil {
		t.Error("DetectVersions() returned nil, want empty slice")
	}
}

// containsSubstring checks if s contains substr.
func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstringHelper(s, substr))
}

func containsSubstringHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
