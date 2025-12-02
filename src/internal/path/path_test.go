package path

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dtvem/dtvem/src/internal/constants"
)

func TestIsInPath(t *testing.T) {
	// Get current PATH
	originalPath := os.Getenv("PATH")

	tests := []struct {
		name      string
		dir       string
		setupPath string
		expected  bool
	}{
		{
			name:      "Directory exists in PATH",
			dir:       "/usr/bin",
			setupPath: "/usr/bin:/usr/local/bin",
			expected:  true,
		},
		{
			name:      "Directory not in PATH",
			dir:       "/nonexistent",
			setupPath: "/usr/bin:/usr/local/bin",
			expected:  false,
		},
		{
			name:      "Empty PATH",
			dir:       "/usr/bin",
			setupPath: "",
			expected:  false,
		},
	}

	// Adjust separator for Windows
	separator := ":"
	if runtime.GOOS == constants.OSWindows {
		separator = ";"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test PATH
			testPath := strings.ReplaceAll(tt.setupPath, ":", separator)
			_ = os.Setenv("PATH", testPath)

			// Clean the directory path for comparison
			cleanDir := filepath.Clean(tt.dir)
			result := IsInPath(cleanDir)

			if result != tt.expected {
				t.Errorf("IsInPath(%q) with PATH=%q = %v, want %v",
					cleanDir, testPath, result, tt.expected)
			}
		})
	}

	// Restore original PATH
	_ = os.Setenv("PATH", originalPath)
}

func TestIsInPath_WithSpaces(t *testing.T) {
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	separator := ":"
	if runtime.GOOS == constants.OSWindows {
		separator = ";"
	}

	testDir := "/path with spaces"
	testPath := strings.Join([]string{"/usr/bin", testDir, "/usr/local/bin"}, separator)
	_ = os.Setenv("PATH", testPath)

	if !IsInPath(testDir) {
		t.Errorf("IsInPath(%q) = false, want true (should handle spaces in paths)", testDir)
	}
}

func TestShimsDir(t *testing.T) {
	result := ShimsDir()

	// Should return a non-empty path
	if result == "" {
		t.Error("ShimsDir() returned empty string")
	}

	// Should contain .dtvem
	if !strings.Contains(result, ".dtvem") {
		t.Errorf("ShimsDir() = %q, should contain '.dtvem'", result)
	}

	// Should end with 'shims'
	if !strings.HasSuffix(result, "shims") {
		t.Errorf("ShimsDir() = %q, should end with 'shims'", result)
	}

	// Should be an absolute path
	if !filepath.IsAbs(result) {
		t.Errorf("ShimsDir() = %q, should be an absolute path", result)
	}
}
