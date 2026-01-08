package version

import (
	"testing"
)

func TestIsPartialVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		// Single component (partial)
		{"22", true},
		{"v22", true},
		{"3", true},

		// Two components (partial)
		{"22.15", true},
		{"v22.15", true},
		{"14.21", true},

		// Three components (full)
		{"22.15.0", false},
		{"v22.15.0", false},
		{"3.11.0", false},

		// More than three components (still considered full)
		{"22.15.0.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := IsPartialVersion(tt.input)
			if result != tt.expected {
				t.Errorf("IsPartialVersion(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolvePartialVersion_MajorOnly(t *testing.T) {
	available := []string{
		"22.0.0",
		"22.5.0",
		"22.15.0",
		"22.15.1",
		"21.0.0",
		"20.10.0",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"22", "22.15.1"},  // Should find highest 22.x.x
		{"v22", "22.15.1"}, // Should handle v prefix
		{"21", "21.0.0"},   // Single match
		{"20", "20.10.0"},  // Different major
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ResolvePartialVersion(tt.input, available)
			if err != nil {
				t.Errorf("ResolvePartialVersion(%q) returned error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ResolvePartialVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolvePartialVersion_MajorMinor(t *testing.T) {
	available := []string{
		"14.21.0",
		"14.21.3",
		"14.20.0",
		"14.20.1",
		"14.19.0",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"14.21", "14.21.3"}, // Should find highest 14.21.x
		{"14.20", "14.20.1"}, // Different minor
		{"14.19", "14.19.0"}, // Single match
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ResolvePartialVersion(tt.input, available)
			if err != nil {
				t.Errorf("ResolvePartialVersion(%q) returned error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ResolvePartialVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolvePartialVersion_FullVersion(t *testing.T) {
	available := []string{
		"22.15.0",
		"22.15.1",
	}

	// Full version should be returned as-is (passthrough)
	tests := []struct {
		input    string
		expected string
	}{
		{"22.15.0", "22.15.0"},
		{"v22.15.0", "22.15.0"},  // v prefix stripped
		{"99.99.99", "99.99.99"}, // Not in available list, but still returned (caller validates)
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ResolvePartialVersion(tt.input, available)
			if err != nil {
				t.Errorf("ResolvePartialVersion(%q) returned error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ResolvePartialVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolvePartialVersion_NoMatch(t *testing.T) {
	available := []string{
		"22.0.0",
		"21.0.0",
	}

	tests := []struct {
		input string
	}{
		{"99"},    // Major not found
		{"22.99"}, // Minor not found
		{"20"},    // Major not found
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ResolvePartialVersion(tt.input, available)
			if err == nil {
				t.Errorf("ResolvePartialVersion(%q) expected error, got nil", tt.input)
			}
		})
	}
}

func TestResolvePartialVersion_EmptyList(t *testing.T) {
	_, err := ResolvePartialVersion("22", []string{})
	if err == nil {
		t.Error("ResolvePartialVersion with empty list expected error, got nil")
	}
}

func TestResolvePartialVersion_PythonVersions(t *testing.T) {
	// Test with Python-style versions
	available := []string{
		"3.9.0",
		"3.9.18",
		"3.10.0",
		"3.10.13",
		"3.11.0",
		"3.11.7",
		"3.12.0",
		"3.12.1",
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"3", "3.12.1"},    // Latest 3.x.x
		{"3.11", "3.11.7"}, // Latest 3.11.x
		{"3.9", "3.9.18"},  // Latest 3.9.x
		{"3.12", "3.12.1"}, // Latest 3.12.x
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := ResolvePartialVersion(tt.input, available)
			if err != nil {
				t.Errorf("ResolvePartialVersion(%q) returned error: %v", tt.input, err)
				return
			}
			if result != tt.expected {
				t.Errorf("ResolvePartialVersion(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestMatchesPartial(t *testing.T) {
	tests := []struct {
		version      string
		partialParts []string
		expected     bool
	}{
		{"22.15.0", []string{"22"}, true},
		{"22.15.0", []string{"22", "15"}, true},
		{"22.15.0", []string{"22", "15", "0"}, true},
		{"22.15.0", []string{"22", "16"}, false},
		{"22.15.0", []string{"21"}, false},
		{"v22.15.0", []string{"22"}, true}, // v prefix handled
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			result := matchesPartial(tt.version, tt.partialParts)
			if result != tt.expected {
				t.Errorf("matchesPartial(%q, %v) = %v, want %v", tt.version, tt.partialParts, result, tt.expected)
			}
		})
	}
}

func TestSortVersionsDesc(t *testing.T) {
	versions := []string{
		"22.0.0",
		"22.15.1",
		"21.0.0",
		"22.15.0",
		"22.5.0",
	}

	sortVersionsDesc(versions)

	expected := []string{
		"22.15.1",
		"22.15.0",
		"22.5.0",
		"22.0.0",
		"21.0.0",
	}

	for i, v := range versions {
		if v != expected[i] {
			t.Errorf("sortVersionsDesc: position %d = %q, want %q", i, v, expected[i])
		}
	}
}

func TestCompareVersionStrings(t *testing.T) {
	tests := []struct {
		a        string
		b        string
		expected int // >0, <0, or 0
	}{
		{"22.15.1", "22.15.0", 1},  // a > b
		{"22.15.0", "22.15.1", -1}, // a < b
		{"22.15.0", "22.15.0", 0},  // equal
		{"22.15.0", "22.5.0", 1},   // 15 > 5
		{"3.10.0", "3.9.0", 1},     // 10 > 9
		{"22.0.0", "21.99.99", 1},  // major takes precedence
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			result := compareVersionStrings(tt.a, tt.b)
			if (tt.expected > 0 && result <= 0) || (tt.expected < 0 && result >= 0) || (tt.expected == 0 && result != 0) {
				t.Errorf("compareVersionStrings(%q, %q) = %d, want sign of %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}
