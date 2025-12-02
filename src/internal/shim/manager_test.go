package shim

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/dtvem/dtvem/src/internal/constants"
	runtimepkg "github.com/dtvem/dtvem/src/internal/runtime"
)

// mockProvider for testing
type mockProvider struct {
	name  string
	shims []string
}

func (m *mockProvider) Name() string                                                  { return m.name }
func (m *mockProvider) DisplayName() string                                           { return m.name }
func (m *mockProvider) Shims() []string                                               { return m.shims }
func (m *mockProvider) Install(version string) error                                  { return nil }
func (m *mockProvider) Uninstall(version string) error                                { return nil }
func (m *mockProvider) ListInstalled() ([]runtimepkg.InstalledVersion, error)         { return nil, nil }
func (m *mockProvider) ListAvailable() ([]runtimepkg.AvailableVersion, error)         { return nil, nil }
func (m *mockProvider) ExecutablePath(version string) (string, error)                 { return "", nil }
func (m *mockProvider) IsInstalled(version string) (bool, error)                      { return false, nil }
func (m *mockProvider) InstallPath(version string) (string, error)                    { return "", nil }
func (m *mockProvider) GlobalVersion() (string, error)                                { return "", nil }
func (m *mockProvider) SetGlobalVersion(version string) error                         { return nil }
func (m *mockProvider) LocalVersion() (string, error)                                 { return "", nil }
func (m *mockProvider) SetLocalVersion(version string) error                          { return nil }
func (m *mockProvider) CurrentVersion() (string, error)                               { return "", nil }
func (m *mockProvider) DetectInstalled() ([]runtimepkg.DetectedVersion, error)        { return nil, nil }
func (m *mockProvider) GlobalPackages(installPath string) ([]string, error)           { return nil, nil }
func (m *mockProvider) InstallGlobalPackages(version string, packages []string) error { return nil }
func (m *mockProvider) ManualPackageInstallCommand(packages []string) string          { return "" }
func (m *mockProvider) ShouldReshimAfter(shimName string, args []string) bool         { return false }

func TestRuntimeShims(t *testing.T) {
	// Register test providers
	_ = runtimepkg.Register(&mockProvider{
		name:  "python",
		shims: []string{"python", "python3", "pip", "pip3"},
	})
	_ = runtimepkg.Register(&mockProvider{
		name:  "node",
		shims: []string{"node", "npm", "npx"},
	})

	// Cleanup after test
	defer func() {
		_ = runtimepkg.Unregister("python")
		_ = runtimepkg.Unregister("node")
	}()

	tests := []struct {
		name          string
		runtimeName   string
		expectedShims []string
	}{
		{
			name:          "Python shims",
			runtimeName:   "python",
			expectedShims: []string{"python", "python3", "pip", "pip3"},
		},
		{
			name:          "Node.js shims",
			runtimeName:   "node",
			expectedShims: []string{"node", "npm", "npx"},
		},
		{
			name:          "Ruby shims (provider not registered yet)",
			runtimeName:   "ruby",
			expectedShims: []string{"ruby"}, // Default behavior when provider not found
		},
		{
			name:          "Go shims (provider not registered yet)",
			runtimeName:   "go",
			expectedShims: []string{"go"}, // Default behavior when provider not found
		},
		{
			name:          "Unknown runtime defaults to runtime name",
			runtimeName:   "unknown",
			expectedShims: []string{"unknown"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RuntimeShims(tt.runtimeName)

			if !reflect.DeepEqual(result, tt.expectedShims) {
				t.Errorf("RuntimeShims(%q) = %v, want %v",
					tt.runtimeName, result, tt.expectedShims)
			}
		})
	}
}

func TestRuntimeShims_CaseInsensitive(t *testing.T) {
	// Test that runtime names are case-sensitive (current behavior)
	result := RuntimeShims("Python") // capital P
	expected := []string{"Python"}   // Should default to runtime name

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("RuntimeShims(\"Python\") = %v, want %v", result, expected)
	}
}

// Complex tests for shim manager operations

func TestManager_CreateShim(t *testing.T) {
	// Create temp directories for shim source and destination
	tmpRoot := t.TempDir()

	// Create a fake shim source executable
	shimSourcePath := filepath.Join(tmpRoot, "dtvem-shim")
	if runtime.GOOS == constants.OSWindows {
		shimSourcePath += constants.ExtExe
	}
	if err := os.WriteFile(shimSourcePath, []byte("fake shim content"), 0755); err != nil {
		t.Fatalf("Failed to create fake shim: %v", err)
	}

	// Create shims directory
	shimsDir := filepath.Join(tmpRoot, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims directory: %v", err)
	}

	// Override environment to use temp directory
	t.Setenv("HOME", tmpRoot)
	t.Setenv("USERPROFILE", tmpRoot)
	t.Setenv("DTVEM_ROOT", tmpRoot)

	// Create a shim
	shimName := "python"
	if runtime.GOOS == constants.OSWindows {
		shimName += constants.ExtExe
	}

	expectedShimPath := filepath.Join(shimsDir, shimName)

	// Note: We test copyFile directly rather than Manager.CreateShim
	// because CreateShim uses config.GetShimPath which needs complex setup
	err := copyFile(shimSourcePath, expectedShimPath)
	if err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	// Verify shim was created
	if _, err := os.Stat(expectedShimPath); os.IsNotExist(err) {
		t.Error("Shim file was not created")
	}

	// Verify content matches source
	sourceContent, _ := os.ReadFile(shimSourcePath)
	destContent, _ := os.ReadFile(expectedShimPath)

	if !reflect.DeepEqual(sourceContent, destContent) {
		t.Error("Shim content does not match source")
	}
}

func TestManager_CreateShims(t *testing.T) {
	tmpRoot := t.TempDir()

	// Create fake shim source
	shimSourcePath := filepath.Join(tmpRoot, "dtvem-shim")
	if runtime.GOOS == constants.OSWindows {
		shimSourcePath += constants.ExtExe
	}
	if err := os.WriteFile(shimSourcePath, []byte("fake shim"), 0755); err != nil {
		t.Fatalf("Failed to create fake shim: %v", err)
	}

	// Create shims directory
	shimsDir := filepath.Join(tmpRoot, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims directory: %v", err)
	}

	// Test creating multiple shims (using copyFile directly)
	shimNames := []string{"python", "node", "ruby"}
	for _, name := range shimNames {
		destPath := filepath.Join(shimsDir, name)
		if runtime.GOOS == constants.OSWindows {
			destPath += constants.ExtExe
		}
		if err := copyFile(shimSourcePath, destPath); err != nil {
			t.Errorf("Failed to create shim %s: %v", name, err)
		}
	}

	// Verify all were created
	for _, name := range shimNames {
		destPath := filepath.Join(shimsDir, name)
		if runtime.GOOS == constants.OSWindows {
			destPath += constants.ExtExe
		}
		if _, err := os.Stat(destPath); os.IsNotExist(err) {
			t.Errorf("Shim %s was not created", name)
		}
	}
}

func TestCopyFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Test various file sizes
	tests := []struct {
		name    string
		content []byte
	}{
		{
			name:    "small file",
			content: []byte("hello world"),
		},
		{
			name:    "empty file",
			content: []byte(""),
		},
		{
			name:    "large file",
			content: make([]byte, 1024*10), // 10KB
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := filepath.Join(tmpDir, "source")
			dst := filepath.Join(tmpDir, "dest")

			// Write source file
			if err := os.WriteFile(src, tt.content, 0644); err != nil {
				t.Fatalf("Failed to create source file: %v", err)
			}

			// Copy file
			if err := copyFile(src, dst); err != nil {
				t.Fatalf("copyFile() error: %v", err)
			}

			// Verify destination exists
			if _, err := os.Stat(dst); os.IsNotExist(err) {
				t.Fatal("Destination file was not created")
			}

			// Verify content matches
			destContent, err := os.ReadFile(dst)
			if err != nil {
				t.Fatalf("Failed to read destination: %v", err)
			}

			if !reflect.DeepEqual(tt.content, destContent) {
				t.Error("File content does not match after copy")
			}

			// Clean up for next test
			_ = os.Remove(src)
			_ = os.Remove(dst)
		})
	}
}

func TestCopyFile_Errors(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		setupFunc func() (src, dst string)
	}{
		{
			name: "source file does not exist",
			setupFunc: func() (string, string) {
				return filepath.Join(tmpDir, "nonexistent"), filepath.Join(tmpDir, "dest")
			},
		},
		{
			name: "destination directory does not exist",
			setupFunc: func() (string, string) {
				src := filepath.Join(tmpDir, "source")
				_ = os.WriteFile(src, []byte("content"), 0644)
				return src, filepath.Join(tmpDir, "nonexistent", "dest")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst := tt.setupFunc()

			err := copyFile(src, dst)
			if err == nil {
				t.Error("copyFile() should return error for invalid input")
			}
		})
	}
}

func TestRuntimeShims_AllKnownRuntimes(t *testing.T) {
	// Verify all known runtimes have shim mappings
	knownRuntimes := []string{"python", "node", "ruby", "go"}

	for _, runtime := range knownRuntimes {
		shims := RuntimeShims(runtime)
		if len(shims) == 0 {
			t.Errorf("RuntimeShims(%q) returned empty slice", runtime)
		}

		// Verify at least the runtime name itself is in shims
		found := false
		for _, shim := range shims {
			if shim == runtime {
				found = true
				break
			}
		}

		if !found && runtime != "python" { // python might not include "python" if it only has "python3"
			t.Errorf("RuntimeShims(%q) does not include the runtime name itself", runtime)
		}
	}
}
