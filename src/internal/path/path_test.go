package path

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
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

	// Should contain 'dtvem' (either .dtvem or .local/share/dtvem on Linux)
	if !strings.Contains(result, "dtvem") {
		t.Errorf("ShimsDir() = %q, should contain 'dtvem'", result)
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

func TestShimsDir_WithDTVEMROOT(t *testing.T) {
	// Save original environment
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
	}()

	// Set custom DTVEM_ROOT
	customRoot := filepath.Join(os.TempDir(), "custom-dtvem-root")
	_ = os.Setenv("DTVEM_ROOT", customRoot)

	result := ShimsDir()
	expected := filepath.Join(customRoot, "shims")
	if result != expected {
		t.Errorf("ShimsDir() with DTVEM_ROOT=%q = %q, want %q", customRoot, result, expected)
	}
}

func TestShimsDir_NonLinux_WithXDG(t *testing.T) {
	// On non-Linux platforms, verify that XDG_DATA_HOME is respected when set
	if runtime.GOOS == constants.OSLinux {
		t.Skip("This test only runs on non-Linux platforms")
	}

	// Save original environment
	originalRoot := os.Getenv("DTVEM_ROOT")
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		if originalXDG != "" {
			_ = os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			_ = os.Unsetenv("XDG_DATA_HOME")
		}
	}()

	// Clear DTVEM_ROOT and set XDG_DATA_HOME
	_ = os.Unsetenv("DTVEM_ROOT")
	customXDG := filepath.Join(os.TempDir(), "custom-xdg-data")
	_ = os.Setenv("XDG_DATA_HOME", customXDG)

	result := ShimsDir()
	expected := filepath.Join(customXDG, "dtvem", "shims")

	if result != expected {
		t.Errorf("ShimsDir() on %s should use XDG_DATA_HOME when set, got %q, want %q",
			runtime.GOOS, result, expected)
	}
}

func TestShimsDir_NonLinux_WithoutXDG(t *testing.T) {
	// On non-Linux platforms, verify that ~/.dtvem/shims is used when XDG_DATA_HOME is not set
	if runtime.GOOS == constants.OSLinux {
		t.Skip("This test only runs on non-Linux platforms")
	}

	// Save original environment
	originalRoot := os.Getenv("DTVEM_ROOT")
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		if originalXDG != "" {
			_ = os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			_ = os.Unsetenv("XDG_DATA_HOME")
		}
	}()

	// Clear both DTVEM_ROOT and XDG_DATA_HOME
	_ = os.Unsetenv("DTVEM_ROOT")
	_ = os.Unsetenv("XDG_DATA_HOME")

	result := ShimsDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".dtvem", "shims")

	if result != expected {
		t.Errorf("ShimsDir() on %s without XDG_DATA_HOME should use ~/.dtvem/shims, got %q, want %q",
			runtime.GOOS, result, expected)
	}
}

func TestShimsDir_DTVEMRootOverridesXDG(t *testing.T) {
	// Verify that DTVEM_ROOT takes precedence over XDG_DATA_HOME
	originalRoot := os.Getenv("DTVEM_ROOT")
	originalXDG := os.Getenv("XDG_DATA_HOME")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		if originalXDG != "" {
			_ = os.Setenv("XDG_DATA_HOME", originalXDG)
		} else {
			_ = os.Unsetenv("XDG_DATA_HOME")
		}
	}()

	// Set both DTVEM_ROOT and XDG_DATA_HOME
	customRoot := filepath.Join(os.TempDir(), "custom-dtvem-root")
	_ = os.Setenv("DTVEM_ROOT", customRoot)
	_ = os.Setenv("XDG_DATA_HOME", filepath.Join(os.TempDir(), "should-be-ignored"))

	result := ShimsDir()
	expected := filepath.Join(customRoot, "shims")

	if result != expected {
		t.Errorf("ShimsDir() with DTVEM_ROOT set should return DTVEM_ROOT/shims, got %q, want %q",
			result, expected)
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

func TestIsDtvemShimsPath(t *testing.T) {
	// Platform-specific path separator handling: filepath.Join produces
	// backslashes on Windows and forward slashes on Unix, which matches what
	// real PATH entries look like on each platform.
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "leading-dot dtvem under home",
			path: filepath.Join("C:", "Users", "testuser", ".dtvem", "shims"),
			want: true,
		},
		{
			name: "no-dot dtvem under XDG data home",
			path: filepath.Join("C:", "Users", "testuser", ".local", "share", "dtvem", "shims"),
			want: true,
		},
		{
			name: "unix style leading-dot",
			path: "/home/testuser/.dtvem/shims",
			want: true,
		},
		{
			name: "unix style XDG",
			path: "/home/testuser/.local/share/dtvem/shims",
			want: true,
		},
		{
			name: "trailing slash is normalized",
			path: "/home/testuser/.dtvem/shims/",
			want: true,
		},
		{
			name: "shims under non-dtvem parent does not match",
			path: "/home/testuser/something/shims",
			want: false,
		},
		{
			name: "dtvem dir without shims leaf does not match",
			path: "/home/testuser/.dtvem/bin",
			want: false,
		},
		{
			name: "empty string",
			path: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsDtvemShimsPath(tt.path)
			if got != tt.want {
				t.Errorf("IsDtvemShimsPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsDtvemShimsPath_WindowsCaseInsensitive(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Windows-only: case-insensitive path matching")
	}

	cases := []string{
		`C:\Users\testuser\.DTVEM\Shims`,
		`C:\Users\testuser\.local\share\DTVEM\SHIMS`,
		`C:\Users\testuser\.Dtvem\shims`,
	}
	for _, p := range cases {
		if !IsDtvemShimsPath(p) {
			t.Errorf("IsDtvemShimsPath(%q) = false, want true (Windows case-insensitive)", p)
		}
	}
}

func TestFindStaleShimsEntries(t *testing.T) {
	// Build paths that look right on the current platform so the
	// case-insensitive comparison logic exercises real separators.
	currentXDG := filepath.Join("C:", "Users", "testuser", ".local", "share", "dtvem", "shims")
	staleHome := filepath.Join("C:", "Users", "testuser", ".dtvem", "shims")
	unrelated := filepath.Join("C:", "Windows", "System32")

	tests := []struct {
		name    string
		entries []string
		current string
		want    []string
	}{
		{
			name:    "stale leading-dot entry alongside current XDG",
			entries: []string{currentXDG, unrelated, staleHome},
			current: currentXDG,
			want:    []string{staleHome},
		},
		{
			name:    "no stale entries when only current is present",
			entries: []string{currentXDG, unrelated},
			current: currentXDG,
			want:    nil,
		},
		{
			name:    "current dir is the leading-dot variant",
			entries: []string{staleHome, currentXDG, unrelated},
			current: staleHome,
			want:    []string{currentXDG},
		},
		{
			name:    "empty entries are skipped",
			entries: []string{"", staleHome, "  "},
			current: currentXDG,
			want:    []string{staleHome},
		},
		{
			name:    "preserves original entry strings (not cleaned)",
			entries: []string{staleHome + string(filepath.Separator), unrelated},
			current: currentXDG,
			want:    []string{staleHome + string(filepath.Separator)},
		},
		{
			name:    "empty current shimsDir returns nil",
			entries: []string{staleHome},
			current: "",
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindStaleShimsEntries(tt.entries, tt.current)
			if len(got) != len(tt.want) {
				t.Fatalf("FindStaleShimsEntries() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("FindStaleShimsEntries()[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestFindStaleShimsEntries_WindowsCaseInsensitive(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Windows-only: case-insensitive comparison")
	}

	current := `C:\Users\testuser\.local\share\dtvem\shims`
	// Same logical path as `current` but with mixed casing — should NOT be
	// flagged stale.
	sameAsCurrentDifferentCase := `C:\Users\TESTUSER\.LOCAL\share\dtvem\SHIMS`
	stale := `C:\Users\testuser\.dtvem\shims`

	got := FindStaleShimsEntries([]string{sameAsCurrentDifferentCase, stale}, current)
	if len(got) != 1 || got[0] != stale {
		t.Errorf("FindStaleShimsEntries() = %v, want exactly [%q]", got, stale)
	}
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

func TestPartitionStaleShimsEntries(t *testing.T) {
	// Use the platform-native separators so the case-insensitive
	// comparison on Windows is exercised against realistic strings.
	current := filepath.Join("C:", "Users", "u", ".local", "share", "dtvem", "shims")
	stale1 := filepath.Join("C:", "Users", "u", ".dtvem", "shims")
	stale2 := filepath.Join("C:", "Users", "u", "old", "dtvem", "shims")
	unrelated := filepath.Join("C:", "Windows", "System32")

	tests := []struct {
		name        string
		entries     []string
		current     string
		wantKept    []string
		wantRemoved []string
	}{
		{
			name:        "current shims dir is kept, stale ones removed",
			entries:     []string{current, stale1, unrelated, stale2},
			current:     current,
			wantKept:    []string{current, unrelated},
			wantRemoved: []string{stale1, stale2},
		},
		{
			name:        "empty entries are dropped from kept too",
			entries:     []string{"", current, "  ", stale1},
			current:     current,
			wantKept:    []string{current},
			wantRemoved: []string{stale1},
		},
		{
			name:        "empty current returns everything as kept",
			entries:     []string{current, stale1, unrelated},
			current:     "",
			wantKept:    []string{current, stale1, unrelated},
			wantRemoved: nil,
		},
		{
			name:        "no entries means no kept and no removed",
			entries:     nil,
			current:     current,
			wantKept:    nil,
			wantRemoved: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kept, removed := PartitionStaleShimsEntries(tt.entries, tt.current)
			if !stringSliceEqual(kept, tt.wantKept) {
				t.Errorf("kept = %v, want %v", kept, tt.wantKept)
			}
			if !stringSliceEqual(removed, tt.wantRemoved) {
				t.Errorf("removed = %v, want %v", removed, tt.wantRemoved)
			}
		})
	}
}

func TestRemoveDtvemMarkerBlocks(t *testing.T) {
	current := "/home/u/.local/share/dtvem/shims"

	tests := []struct {
		name        string
		input       string
		current     string
		wantOut     string
		wantRemoved []string
	}{
		{
			name: "bash export with stale path is removed",
			input: `# some user line
# Added by dtvem
export PATH="/home/u/.dtvem/shims:$PATH"
# another user line`,
			current:     current,
			wantRemoved: []string{"/home/u/.dtvem/shims"},
			wantOut: `# some user line
# another user line`,
		},
		{
			name: "fish set -gx with stale path is removed",
			input: `# Added by dtvem
set -gx PATH "/home/u/.dtvem/shims" $PATH`,
			current:     current,
			wantRemoved: []string{"/home/u/.dtvem/shims"},
			wantOut:     ``,
		},
		{
			name: "current shims dir is kept (not stale)",
			input: `# Added by dtvem
export PATH="/home/u/.local/share/dtvem/shims:$PATH"`,
			current: current,
			wantOut: `# Added by dtvem
export PATH="/home/u/.local/share/dtvem/shims:$PATH"`,
			wantRemoved: nil,
		},
		{
			name: "user-written export without marker is preserved",
			input: `export PATH="/home/u/.dtvem/shims:$PATH"
# Added by dtvem
export PATH="/home/u/.dtvem/shims:$PATH"`,
			current:     current,
			wantRemoved: []string{"/home/u/.dtvem/shims"},
			wantOut:     `export PATH="/home/u/.dtvem/shims:$PATH"`,
		},
		{
			name:        "marker with no following line is preserved",
			input:       `# Added by dtvem`,
			current:     current,
			wantOut:     `# Added by dtvem`,
			wantRemoved: nil,
		},
		{
			name: "multiple stale blocks are all removed",
			input: `# Added by dtvem
export PATH="/home/u/.dtvem/shims:$PATH"
some_user_content=1
# Added by dtvem
export PATH="/home/u/old/dtvem/shims:$PATH"`,
			current:     current,
			wantRemoved: []string{"/home/u/.dtvem/shims", "/home/u/old/dtvem/shims"},
			wantOut:     `some_user_content=1`,
		},
		{
			name: "marker followed by non-dtvem export is preserved",
			input: `# Added by dtvem
echo hello`,
			current: current,
			wantOut: `# Added by dtvem
echo hello`,
			wantRemoved: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOut, gotRemoved := removeDtvemMarkerBlocks(tt.input, tt.current)
			if gotOut != tt.wantOut {
				t.Errorf("out = %q\nwant %q", gotOut, tt.wantOut)
			}
			if !stringSliceEqual(gotRemoved, tt.wantRemoved) {
				t.Errorf("removed = %v, want %v", gotRemoved, tt.wantRemoved)
			}
		})
	}
}

func TestRemoveStaleShimsFromShellConfig_MissingFileIsNoop(t *testing.T) {
	// Calling on a non-existent file should be a silent no-op, since
	// "this user doesn't have a config file at all" is a valid state
	// and not a problem doctor should escalate.
	removed, err := RemoveStaleShimsFromShellConfig(filepath.Join(t.TempDir(), "no-such-file"), "/anything")
	if err != nil {
		t.Errorf("expected nil error for missing file, got %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed entries, got %v", removed)
	}
}

func TestRemoveStaleShimsFromShellConfig_RewritesFile(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".bashrc")
	content := `# user line
# Added by dtvem
export PATH="/home/u/.dtvem/shims:$PATH"
# another user line
`
	if err := os.WriteFile(configFile, []byte(content), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}

	removed, err := RemoveStaleShimsFromShellConfig(configFile, "/home/u/.local/share/dtvem/shims")
	if err != nil {
		t.Fatalf("RemoveStaleShimsFromShellConfig: %v", err)
	}
	if len(removed) != 1 || removed[0] != "/home/u/.dtvem/shims" {
		t.Errorf("removed: got %v, want [/home/u/.dtvem/shims]", removed)
	}

	got, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := `# user line
# another user line
`
	if string(got) != want {
		t.Errorf("rewritten file:\n got %q\nwant %q", got, want)
	}
}

func TestRemoveStaleShimsFromShellConfig_LeavesFileAloneWhenNothingToDo(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".bashrc")
	original := `# user line only`
	if err := os.WriteFile(configFile, []byte(original), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Read mtime so we can assert we didn't rewrite the file.
	before, _ := os.Stat(configFile)

	removed, err := RemoveStaleShimsFromShellConfig(configFile, "/home/u/.local/share/dtvem/shims")
	if err != nil {
		t.Fatalf("RemoveStaleShimsFromShellConfig: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("expected no removed entries, got %v", removed)
	}
	after, _ := os.Stat(configFile)
	// On some filesystems mtime resolution is coarse, so equality is
	// the right check rather than "after >= before".
	if !before.ModTime().Equal(after.ModTime()) {
		t.Errorf("file was rewritten when there was nothing to remove (mtime changed)")
	}
}

func TestQuotedSubstrings(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{`export PATH="/a:$PATH"`, []string{`/a:$PATH`}},
		{`set -gx PATH "/a" $PATH`, []string{`/a`}},
		{`a "b" c "d"`, []string{`b`, `d`}},
		{`no quotes here`, nil},
		{`unterminated "ohno`, nil},
		{`empty "" here`, []string{``}},
	}
	for _, tt := range tests {
		got := quotedSubstrings(tt.in)
		if !stringSliceEqual(got, tt.want) {
			t.Errorf("quotedSubstrings(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

// stringSliceEqual returns true when a and b have the same length and
// identical elements. Defined locally because reflect.DeepEqual treats
// nil and []string{} as different and that distinction is noise in
// these tests.
func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
