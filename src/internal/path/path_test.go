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

func TestLookPathExcludingShims(t *testing.T) {
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	// Create temp directories for testing
	tempDir := t.TempDir()
	systemDir := filepath.Join(tempDir, "system")
	shimsDir := filepath.Join(tempDir, "shims")

	if err := os.MkdirAll(systemDir, 0755); err != nil {
		t.Fatalf("Failed to create system dir: %v", err)
	}
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims dir: %v", err)
	}

	// Create test executables
	execName := "testexec"
	var systemExec, shimsExec string
	if runtime.GOOS == constants.OSWindows {
		systemExec = filepath.Join(systemDir, execName+".exe")
		shimsExec = filepath.Join(shimsDir, execName+".exe")
	} else {
		systemExec = filepath.Join(systemDir, execName)
		shimsExec = filepath.Join(shimsDir, execName)
	}

	// Create dummy executables
	if err := os.WriteFile(systemExec, []byte("system"), 0755); err != nil {
		t.Fatalf("Failed to create system exec: %v", err)
	}
	if err := os.WriteFile(shimsExec, []byte("shim"), 0755); err != nil {
		t.Fatalf("Failed to create shims exec: %v", err)
	}

	t.Run("Finds executable in system dir", func(t *testing.T) {
		separator := ":"
		if runtime.GOOS == constants.OSWindows {
			separator = ";"
		}
		testPath := strings.Join([]string{systemDir}, separator)
		_ = os.Setenv("PATH", testPath)

		result := LookPathExcludingShims(execName)
		if result != systemExec {
			t.Errorf("LookPathExcludingShims(%q) = %q, want %q", execName, result, systemExec)
		}
	})

	t.Run("Returns empty when not found", func(t *testing.T) {
		_ = os.Setenv("PATH", systemDir)

		result := LookPathExcludingShims("nonexistent")
		if result != "" {
			t.Errorf("LookPathExcludingShims(%q) = %q, want empty string", "nonexistent", result)
		}
	})

	t.Run("Returns empty with empty PATH", func(t *testing.T) {
		_ = os.Setenv("PATH", "")

		result := LookPathExcludingShims(execName)
		if result != "" {
			t.Errorf("LookPathExcludingShims(%q) with empty PATH = %q, want empty string", execName, result)
		}
	})
}

func TestLookPathExcludingShims_SkipsShimsDir(t *testing.T) {
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	// Get the actual shims directory that will be excluded
	shimsDir := ShimsDir()

	// Create temp directory for "system" install
	tempDir := t.TempDir()
	systemDir := filepath.Join(tempDir, "system")
	if err := os.MkdirAll(systemDir, 0755); err != nil {
		t.Fatalf("Failed to create system dir: %v", err)
	}

	// Create shims directory if it doesn't exist (for testing)
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims dir: %v", err)
	}

	execName := "lookuptest"
	var systemExec, shimsExec string
	if runtime.GOOS == constants.OSWindows {
		systemExec = filepath.Join(systemDir, execName+".exe")
		shimsExec = filepath.Join(shimsDir, execName+".exe")
	} else {
		systemExec = filepath.Join(systemDir, execName)
		shimsExec = filepath.Join(shimsDir, execName)
	}

	// Create dummy executables
	if err := os.WriteFile(systemExec, []byte("system"), 0755); err != nil {
		t.Fatalf("Failed to create system exec: %v", err)
	}
	if err := os.WriteFile(shimsExec, []byte("shim"), 0755); err != nil {
		t.Fatalf("Failed to create shims exec: %v", err)
	}
	// Clean up shims exec after test
	defer func() { _ = os.Remove(shimsExec) }()

	separator := ":"
	if runtime.GOOS == constants.OSWindows {
		separator = ";"
	}

	t.Run("Skips shims dir and finds system install", func(t *testing.T) {
		// Put shims dir FIRST in PATH, then system dir
		testPath := strings.Join([]string{shimsDir, systemDir}, separator)
		_ = os.Setenv("PATH", testPath)

		result := LookPathExcludingShims(execName)

		// Should find the system exec, NOT the shims exec
		if result != systemExec {
			t.Errorf("LookPathExcludingShims(%q) = %q, want %q (should skip shims dir)", execName, result, systemExec)
		}
	})

	t.Run("Returns empty when only in shims dir", func(t *testing.T) {
		// Put ONLY shims dir in PATH
		_ = os.Setenv("PATH", shimsDir)

		result := LookPathExcludingShims(execName)

		// Should return empty since shims dir is excluded
		if result != "" {
			t.Errorf("LookPathExcludingShims(%q) = %q, want empty (shims dir should be excluded)", execName, result)
		}
	})
}

func TestFindExecutableInDir(t *testing.T) {
	tempDir := t.TempDir()

	execName := "findtest"
	var execPath string
	if runtime.GOOS == constants.OSWindows {
		execPath = filepath.Join(tempDir, execName+".exe")
	} else {
		execPath = filepath.Join(tempDir, execName)
	}

	// Create dummy executable
	if err := os.WriteFile(execPath, []byte("test"), 0755); err != nil {
		t.Fatalf("Failed to create exec: %v", err)
	}

	t.Run("Finds executable", func(t *testing.T) {
		result := findExecutableInDir(tempDir, execName)
		if result != execPath {
			t.Errorf("findExecutableInDir(%q, %q) = %q, want %q", tempDir, execName, result, execPath)
		}
	})

	t.Run("Returns empty for nonexistent", func(t *testing.T) {
		result := findExecutableInDir(tempDir, "nonexistent")
		if result != "" {
			t.Errorf("findExecutableInDir(%q, %q) = %q, want empty", tempDir, "nonexistent", result)
		}
	})

	t.Run("Returns empty for directory with same name", func(t *testing.T) {
		dirName := "isdir"
		var dirPath string
		if runtime.GOOS == constants.OSWindows {
			dirPath = filepath.Join(tempDir, dirName+".exe")
		} else {
			dirPath = filepath.Join(tempDir, dirName)
		}
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			t.Fatalf("Failed to create dir: %v", err)
		}

		result := findExecutableInDir(tempDir, dirName)
		if result != "" {
			t.Errorf("findExecutableInDir should not return directories, got %q", result)
		}
	})
}
