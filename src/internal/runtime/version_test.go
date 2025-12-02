package runtime

import (
	"testing"
)

func TestNewVersion(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple version",
			input:    "3.11.0",
			expected: "3.11.0",
		},
		{
			name:     "version with spaces",
			input:    "  18.16.0  ",
			expected: "  18.16.0  ",
		},
		{
			name:     "empty version",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := NewVersion(tt.input)
			if v.Raw != tt.expected {
				t.Errorf("NewVersion(%q).Raw = %q, want %q", tt.input, v.Raw, tt.expected)
			}
		})
	}
}

func TestVersion_String(t *testing.T) {
	tests := []struct {
		name     string
		version  Version
		expected string
	}{
		{
			name:     "standard version",
			version:  Version{Raw: "3.11.0"},
			expected: "3.11.0",
		},
		{
			name:     "empty version",
			version:  Version{Raw: ""},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.version.String()
			if result != tt.expected {
				t.Errorf("Version.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestVersion_Equal(t *testing.T) {
	tests := []struct {
		name     string
		v1       Version
		v2       Version
		expected bool
	}{
		{
			name:     "equal versions",
			v1:       Version{Raw: "3.11.0"},
			v2:       Version{Raw: "3.11.0"},
			expected: true,
		},
		{
			name:     "different versions",
			v1:       Version{Raw: "3.11.0"},
			v2:       Version{Raw: "3.12.0"},
			expected: false,
		},
		{
			name:     "empty versions",
			v1:       Version{Raw: ""},
			v2:       Version{Raw: ""},
			expected: true,
		},
		{
			name:     "one empty version",
			v1:       Version{Raw: "3.11.0"},
			v2:       Version{Raw: ""},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.v1.Equal(tt.v2)
			if result != tt.expected {
				t.Errorf("Version.Equal() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestInstalledVersion_String(t *testing.T) {
	tests := []struct {
		name     string
		iv       InstalledVersion
		expected string
	}{
		{
			name: "global version",
			iv: InstalledVersion{
				Version:     Version{Raw: "3.11.0"},
				InstallPath: "/path/to/python",
				IsGlobal:    true,
			},
			expected: "3.11.0 (global)",
		},
		{
			name: "non-global version",
			iv: InstalledVersion{
				Version:     Version{Raw: "18.16.0"},
				InstallPath: "/path/to/node",
				IsGlobal:    false,
			},
			expected: "18.16.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.iv.String()
			if result != tt.expected {
				t.Errorf("InstalledVersion.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestDetectedVersion_String(t *testing.T) {
	tests := []struct {
		name     string
		dv       DetectedVersion
		expected string
	}{
		{
			name: "system installation",
			dv: DetectedVersion{
				Version:   "3.11.0",
				Path:      "/usr/bin/python3",
				Source:    "system",
				Validated: true,
			},
			expected: "v3.11.0 (system) /usr/bin/python3",
		},
		{
			name: "nvm installation",
			dv: DetectedVersion{
				Version:   "18.16.0",
				Path:      "/home/user/.nvm/versions/node/v18.16.0/bin/node",
				Source:    "nvm",
				Validated: false,
			},
			expected: "v18.16.0 (nvm) /home/user/.nvm/versions/node/v18.16.0/bin/node",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.dv.String()
			if result != tt.expected {
				t.Errorf("DetectedVersion.String() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseVersions(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "multiple versions",
			input:    []string{"3.11.0", "3.12.0", "18.16.0"},
			expected: []string{"3.11.0", "3.12.0", "18.16.0"},
		},
		{
			name:     "versions with spaces",
			input:    []string{"  3.11.0  ", "3.12.0", "  18.16.0"},
			expected: []string{"3.11.0", "3.12.0", "18.16.0"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single version",
			input:    []string{"3.11.0"},
			expected: []string{"3.11.0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseVersions(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("ParseVersions() returned %d versions, want %d", len(result), len(tt.expected))
			}
			for i, expected := range tt.expected {
				if result[i].Raw != expected {
					t.Errorf("ParseVersions()[%d].Raw = %q, want %q", i, result[i].Raw, expected)
				}
			}
		})
	}
}
