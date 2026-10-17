package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

func TestGetPaths(t *testing.T) {
	paths := DefaultPaths()

	// Verify paths is not nil
	if paths == nil {
		t.Fatal("DefaultPaths() returned nil")
	}

	// Verify all paths are set
	if paths.Root == "" {
		t.Error("Root path is empty")
	}
	if paths.Shims == "" {
		t.Error("Shims path is empty")
	}
	if paths.Versions == "" {
		t.Error("Versions path is empty")
	}
	if paths.Config == "" {
		t.Error("Config path is empty")
	}

	// Verify paths are absolute
	if !filepath.IsAbs(paths.Root) {
		t.Errorf("Root path %q is not absolute", paths.Root)
	}

	// Verify subdirectories are under root
	if !strings.HasPrefix(paths.Shims, paths.Root) {
		t.Errorf("Shims path %q should be under Root %q", paths.Shims, paths.Root)
	}
	if !strings.HasPrefix(paths.Versions, paths.Root) {
		t.Errorf("Versions path %q should be under Root %q", paths.Versions, paths.Root)
	}
	if !strings.HasPrefix(paths.Config, paths.Root) {
		t.Errorf("Config path %q should be under Root %q", paths.Config, paths.Root)
	}
	if !strings.HasPrefix(paths.Temp, paths.Root) {
		t.Errorf("Temp path %q should be under Root %q", paths.Temp, paths.Root)
	}
}

// useTempRoot points DTVEM_ROOT at a fresh temp directory for the duration of
// the test and returns it.
func useTempRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("DTVEM_ROOT", root)
	resetPathsForTesting()
	t.Cleanup(resetPathsForTesting)
	return root
}

func TestStagingPath(t *testing.T) {
	root := useTempRoot(t)

	got := StagingPath("node", "24.22.0")
	want := filepath.Join(root, "tmp", "node-24.22.0")
	if got != want {
		t.Errorf("StagingPath(%q, %q) = %q, want %q", "node", "24.22.0", got, want)
	}
}

func TestStagingPath_SameVolumeAsVersions(t *testing.T) {
	// Installs finish with os.Rename from the staging path to the version
	// path, which fails across drives on Windows (issue #278). Both must
	// resolve under the dtvem root, never the OS temp directory.
	root := useTempRoot(t)

	staging := StagingPath("python", "3.12.0")
	install := RuntimeVersionPath("python", "3.12.0")

	if filepath.VolumeName(staging) != filepath.VolumeName(install) {
		t.Errorf("StagingPath volume %q differs from RuntimeVersionPath volume %q",
			filepath.VolumeName(staging), filepath.VolumeName(install))
	}
	if !strings.HasPrefix(staging, root) {
		t.Errorf("StagingPath() = %q, should be under root %q", staging, root)
	}
	if strings.HasPrefix(staging, DefaultPaths().Versions) {
		t.Errorf("StagingPath() = %q, should not be under the versions directory", staging)
	}
}

func TestCreateStagingDir(t *testing.T) {
	useTempRoot(t)

	dir, err := CreateStagingDir("ruby", "3.3.0")
	if err != nil {
		t.Fatalf("CreateStagingDir() error: %v", err)
	}
	if want := StagingPath("ruby", "3.3.0"); dir != want {
		t.Errorf("CreateStagingDir() = %q, want %q", dir, want)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("staging directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("staging path %q is not a directory", dir)
	}
}

func TestCreateStagingDir_RemovesLeftovers(t *testing.T) {
	useTempRoot(t)

	stale := filepath.Join(StagingPath("node", "22.0.0"), "extracted", "node.exe")
	if err := os.MkdirAll(filepath.Dir(stale), 0755); err != nil {
		t.Fatalf("failed to create stale directory: %v", err)
	}
	if err := os.WriteFile(stale, []byte("stale"), 0644); err != nil {
		t.Fatalf("failed to create stale file: %v", err)
	}

	dir, err := CreateStagingDir("node", "22.0.0")
	if err != nil {
		t.Fatalf("CreateStagingDir() error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read staging directory: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("CreateStagingDir() left %d stale entries in %q", len(entries), dir)
	}
}

func TestRuntimeVersionPath(t *testing.T) {
	tests := []struct {
		name        string
		runtimeName string
		version     string
	}{
		{
			name:        "Python version path",
			runtimeName: "python",
			version:     "3.11.0",
		},
		{
			name:        "Node.js version path",
			runtimeName: "node",
			version:     "18.16.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RuntimeVersionPath(tt.runtimeName, tt.version)

			// Should contain runtime name
			if !strings.Contains(result, tt.runtimeName) {
				t.Errorf("RuntimeVersionPath(%q, %q) = %q, should contain %q",
					tt.runtimeName, tt.version, result, tt.runtimeName)
			}

			// Should contain version
			if !strings.Contains(result, tt.version) {
				t.Errorf("RuntimeVersionPath(%q, %q) = %q, should contain %q",
					tt.runtimeName, tt.version, result, tt.version)
			}

			// Should be absolute path
			if !filepath.IsAbs(result) {
				t.Errorf("RuntimeVersionPath(%q, %q) = %q, should be absolute",
					tt.runtimeName, tt.version, result)
			}
		})
	}
}

func TestGlobalConfigPath(t *testing.T) {
	result := GlobalConfigPath()

	// Should not be empty
	if result == "" {
		t.Error("GlobalConfigPath() returned empty string")
	}

	// Should end with 'runtimes.json'
	if !strings.HasSuffix(result, RuntimesFileName) {
		t.Errorf("GlobalConfigPath() = %q, should end with %q", result, RuntimesFileName)
	}

	// Should be absolute path
	if !filepath.IsAbs(result) {
		t.Errorf("GlobalConfigPath() = %q, should be absolute", result)
	}

	// Should contain 'config'
	if !strings.Contains(result, "config") {
		t.Errorf("GlobalConfigPath() = %q, should contain 'config'", result)
	}
}

func TestShimPath(t *testing.T) {
	tests := []struct {
		name     string
		shimName string
		wantExt  string
	}{
		{
			name:     "Python shim",
			shimName: "python",
			wantExt:  "",
		},
		{
			name:     "Node shim",
			shimName: "node",
			wantExt:  "",
		},
	}

	// Determine expected extension based on OS
	expectedExt := ""
	if runtime.GOOS == "windows" {
		expectedExt = ".exe"
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ShimPath(tt.shimName)

			// Should contain shim name
			if !strings.Contains(result, tt.shimName) {
				t.Errorf("ShimPath(%q) = %q, should contain %q",
					tt.shimName, result, tt.shimName)
			}

			// Should have .exe extension on Windows
			if runtime.GOOS == "windows" {
				if !strings.HasSuffix(result, ".exe") {
					t.Errorf("ShimPath(%q) = %q, should end with '.exe' on Windows",
						tt.shimName, result)
				}
			}

			// Verify correct extension
			actualExt := filepath.Ext(result)
			if actualExt != expectedExt {
				t.Errorf("ShimPath(%q) extension = %q, want %q on %s",
					tt.shimName, actualExt, expectedExt, runtime.GOOS)
			}

			// Should be absolute path
			if !filepath.IsAbs(result) {
				t.Errorf("ShimPath(%q) = %q, should be absolute",
					tt.shimName, result)
			}
		})
	}
}

// resetPathsForTesting resets the paths singleton for testing purposes.
// This allows tests to verify behavior with different environment configurations.
func resetPathsForTesting() {
	defaultPaths = nil
	pathsOnce = sync.Once{}
}

func TestGetRootDir_WithEnvironmentVariable(t *testing.T) {
	// Save original environment
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		// Reset paths so it reinitializes
		resetPathsForTesting()
	}()

	// Set custom DTVEM_ROOT
	customRoot := "/custom/dtvem/path"
	_ = os.Setenv("DTVEM_ROOT", customRoot)

	// Reset paths to force reinitialization
	resetPathsForTesting()

	// Test that getRootDir respects DTVEM_ROOT
	result := getRootDir()
	if result != customRoot {
		t.Errorf("getRootDir() with DTVEM_ROOT=%q = %q, want %q",
			customRoot, result, customRoot)
	}
}

func TestConfigConstants(t *testing.T) {
	// Test LocalConfigDirName
	expectedDir := ".dtvem"
	if LocalConfigDirName != expectedDir {
		t.Errorf("LocalConfigDirName = %q, want %q", LocalConfigDirName, expectedDir)
	}

	// Test RuntimesFileName
	expectedFile := "runtimes.json"
	if RuntimesFileName != expectedFile {
		t.Errorf("RuntimesFileName = %q, want %q", RuntimesFileName, expectedFile)
	}
}

func TestLocalConfigPath(t *testing.T) {
	result := LocalConfigPath()

	// Should not be empty
	if result == "" {
		t.Error("LocalConfigPath() returned empty string")
	}

	// Should contain .dtvem
	if !strings.Contains(result, LocalConfigDirName) {
		t.Errorf("LocalConfigPath() = %q, should contain %q", result, LocalConfigDirName)
	}

	// Should end with runtimes.json
	if !strings.HasSuffix(result, RuntimesFileName) {
		t.Errorf("LocalConfigPath() = %q, should end with %q", result, RuntimesFileName)
	}
}

func TestDefaultPaths_ConcurrentAccess(t *testing.T) {
	// Reset to ensure clean state
	resetPathsForTesting()
	defer resetPathsForTesting()

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)

	// Channel to collect results
	results := make(chan *Paths, goroutines)

	// Launch multiple goroutines to call DefaultPaths concurrently
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			results <- DefaultPaths()
		}()
	}

	wg.Wait()
	close(results)

	// Collect all results
	var first *Paths
	for paths := range results {
		if first == nil {
			first = paths
		} else {
			// All goroutines should receive the same pointer
			if paths != first {
				t.Errorf("DefaultPaths() returned different pointers: %p vs %p", first, paths)
			}
		}
	}

	// Verify the paths are valid
	if first == nil {
		t.Fatal("DefaultPaths() returned nil")
	}
	if first.Root == "" {
		t.Error("Root path is empty")
	}
}

func TestGetXDGDataPath(t *testing.T) {
	home := "/home/testuser"

	tests := []struct {
		name           string
		xdgDataHome    string
		expectedSuffix string
	}{
		{
			name:           "XDG_DATA_HOME set",
			xdgDataHome:    "/custom/data",
			expectedSuffix: filepath.Join("/custom/data", "dtvem"),
		},
		{
			name:           "XDG_DATA_HOME empty - use default",
			xdgDataHome:    "",
			expectedSuffix: filepath.Join(home, ".local", "share", "dtvem"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore XDG_DATA_HOME
			originalXDG := os.Getenv("XDG_DATA_HOME")
			defer func() {
				if originalXDG != "" {
					_ = os.Setenv("XDG_DATA_HOME", originalXDG)
				} else {
					_ = os.Unsetenv("XDG_DATA_HOME")
				}
			}()

			if tt.xdgDataHome != "" {
				_ = os.Setenv("XDG_DATA_HOME", tt.xdgDataHome)
			} else {
				_ = os.Unsetenv("XDG_DATA_HOME")
			}

			result := getXDGDataPath(home)
			if result != tt.expectedSuffix {
				t.Errorf("getXDGDataPath(%q) = %q, want %q", home, result, tt.expectedSuffix)
			}
		})
	}
}

func TestGetRootDir_XDGOnLinux(t *testing.T) {
	// This test verifies the XDG behavior on Linux
	// On other platforms, it verifies that XDG is NOT used
	if runtime.GOOS != constants.OSLinux {
		t.Skip("XDG tests only run on Linux")
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
		resetPathsForTesting()
	}()

	// Clear DTVEM_ROOT to test XDG behavior
	_ = os.Unsetenv("DTVEM_ROOT")

	// Test with custom XDG_DATA_HOME
	customXDG := "/tmp/custom-xdg-data"
	_ = os.Setenv("XDG_DATA_HOME", customXDG)
	resetPathsForTesting()

	result := getRootDir()
	expected := filepath.Join(customXDG, "dtvem")
	if result != expected {
		t.Errorf("getRootDir() with XDG_DATA_HOME=%q = %q, want %q", customXDG, result, expected)
	}

	// Test with XDG_DATA_HOME unset (should use default)
	_ = os.Unsetenv("XDG_DATA_HOME")
	resetPathsForTesting()

	result = getRootDir()
	home, _ := os.UserHomeDir()
	expected = filepath.Join(home, ".local", "share", "dtvem")
	if result != expected {
		t.Errorf("getRootDir() with XDG_DATA_HOME unset = %q, want %q", result, expected)
	}
}

func TestGetRootDir_NonLinux_WithXDG(t *testing.T) {
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
		resetPathsForTesting()
	}()

	// Clear DTVEM_ROOT and set XDG_DATA_HOME
	_ = os.Unsetenv("DTVEM_ROOT")
	customXDG := "/custom/xdg/data"
	_ = os.Setenv("XDG_DATA_HOME", customXDG)
	resetPathsForTesting()

	result := getRootDir()
	expected := filepath.Join(customXDG, "dtvem")

	if result != expected {
		t.Errorf("getRootDir() on %s should use XDG_DATA_HOME when set, got %q, want %q",
			runtime.GOOS, result, expected)
	}
}

func TestGetRootDir_NonLinux_WithoutXDG(t *testing.T) {
	// On non-Linux platforms, verify that ~/.dtvem is used when XDG_DATA_HOME is not set
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
		resetPathsForTesting()
	}()

	// Clear both DTVEM_ROOT and XDG_DATA_HOME
	_ = os.Unsetenv("DTVEM_ROOT")
	_ = os.Unsetenv("XDG_DATA_HOME")
	resetPathsForTesting()

	result := getRootDir()
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".dtvem")

	if result != expected {
		t.Errorf("getRootDir() on %s without XDG_DATA_HOME should use ~/.dtvem, got %q, want %q",
			runtime.GOOS, result, expected)
	}
}

func TestGetRootDir_DTVEMRootOverridesXDG(t *testing.T) {
	// Verify that DTVEM_ROOT takes precedence over XDG_DATA_HOME on all platforms
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
		resetPathsForTesting()
	}()

	// Set both DTVEM_ROOT and XDG_DATA_HOME
	customRoot := "/custom/dtvem/root"
	_ = os.Setenv("DTVEM_ROOT", customRoot)
	_ = os.Setenv("XDG_DATA_HOME", "/should/be/ignored")
	resetPathsForTesting()

	result := getRootDir()
	if result != customRoot {
		t.Errorf("getRootDir() with DTVEM_ROOT set should return DTVEM_ROOT, got %q, want %q",
			result, customRoot)
	}
}
