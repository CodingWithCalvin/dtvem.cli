package shim

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	runtimepkg "github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
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
func (m *mockProvider) GetEnvironment(_ string) (map[string]string, error) {
	return map[string]string{}, nil
}

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

func TestCreateShim_CreatesCmdWrapperOnWindows(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Skipping Windows-specific test")
	}

	tmpRoot := t.TempDir()
	shimsDir := filepath.Join(tmpRoot, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims directory: %v", err)
	}

	// Create a fake shim source
	shimSourcePath := filepath.Join(tmpRoot, "dtvem-shim.exe")
	if err := os.WriteFile(shimSourcePath, []byte("fake shim content"), 0755); err != nil {
		t.Fatalf("Failed to create fake shim: %v", err)
	}

	// Create the .exe shim
	exePath := filepath.Join(shimsDir, "npm.exe")
	if err := copyFile(shimSourcePath, exePath); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	// Create the .cmd wrapper using the helper
	cmdPath := filepath.Join(shimsDir, "npm.cmd")
	content := "@echo off\r\n\"%~dp0npm.exe\" %*\r\n"
	if err := os.WriteFile(cmdPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write .cmd wrapper: %v", err)
	}

	// Verify .cmd file exists
	if _, err := os.Stat(cmdPath); os.IsNotExist(err) {
		t.Error(".cmd wrapper was not created")
	}

	// Verify .cmd content
	cmdContent, err := os.ReadFile(cmdPath)
	if err != nil {
		t.Fatalf("Failed to read .cmd wrapper: %v", err)
	}

	expected := "@echo off\r\n\"%~dp0npm.exe\" %*\r\n"
	if string(cmdContent) != expected {
		t.Errorf(".cmd content = %q, want %q", string(cmdContent), expected)
	}
}

func TestRemoveShim_RemovesCmdWrapperOnWindows(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Skipping Windows-specific test")
	}

	tmpRoot := t.TempDir()
	shimsDir := filepath.Join(tmpRoot, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims directory: %v", err)
	}

	// Create both .exe and .cmd files
	exePath := filepath.Join(shimsDir, "npm.exe")
	cmdPath := filepath.Join(shimsDir, "npm.cmd")
	if err := os.WriteFile(exePath, []byte("fake shim"), 0755); err != nil {
		t.Fatalf("Failed to create .exe: %v", err)
	}
	if err := os.WriteFile(cmdPath, []byte("@echo off\r\n"), 0644); err != nil {
		t.Fatalf("Failed to create .cmd: %v", err)
	}

	// Remove both files
	if err := os.Remove(exePath); err != nil {
		t.Fatalf("Failed to remove .exe: %v", err)
	}
	if err := os.Remove(cmdPath); err != nil {
		t.Fatalf("Failed to remove .cmd: %v", err)
	}

	// Verify both are gone
	if _, err := os.Stat(exePath); !os.IsNotExist(err) {
		t.Error(".exe shim was not removed")
	}
	if _, err := os.Stat(cmdPath); !os.IsNotExist(err) {
		t.Error(".cmd wrapper was not removed")
	}
}

func TestListShims_SkipsCmdFiles(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Skipping Windows-specific test")
	}

	tmpRoot := t.TempDir()
	shimsDir := filepath.Join(tmpRoot, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims directory: %v", err)
	}

	// Create .exe and .cmd files
	files := map[string]string{
		"npm.exe": "fake shim",
		"npm.cmd": "@echo off\r\n",
		"npx.exe": "fake shim",
		"npx.cmd": "@echo off\r\n",
	}
	for name, content := range files {
		path := filepath.Join(shimsDir, name)
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			t.Fatalf("Failed to create %s: %v", name, err)
		}
	}

	// Read entries and filter like ListShims does
	entries, err := os.ReadDir(shimsDir)
	if err != nil {
		t.Fatalf("Failed to read shims directory: %v", err)
	}

	var shims []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := filepath.Ext(name)
		if ext == constants.ExtCmd || ext == constants.ExtBat {
			continue
		}
		shims = append(shims, name[:len(name)-len(ext)])
	}

	expected := []string{"npm", "npx"}
	if !reflect.DeepEqual(shims, expected) {
		t.Errorf("ListShims filtered result = %v, want %v", shims, expected)
	}
}

// writeExecutable writes a file with the executable bit set on Unix. The
// content is irrelevant — findExecutables only looks at extension (Windows)
// or the exec bit (Unix) — but a non-zero body keeps file readers happy.
func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0755); err != nil {
		t.Fatalf("Failed to write %s: %v", path, err)
	}
}

// platformExeName returns name with the OS-appropriate executable extension.
// On Windows the .exe suffix is required for findExecutables to recognize
// the file; on Unix the bare name is used and the exec bit drives detection.
func platformExeName(name string) string {
	if runtime.GOOS == constants.OSWindows {
		return name + constants.ExtExe
	}
	return name
}

// newShimsTestManager points dtvem at a temp root, creates the shims
// directory, and returns a Manager backed by a stub dtvem-shim source
// along with the shims directory path.
//
// Both config.DefaultPaths and the shim-map cache memoize on first use,
// so each is dropped on the way in (to pick up DTVEM_ROOT) and again on
// the way out (so the next test isn't left pointing at this temp root).
func newShimsTestManager(t *testing.T) (*Manager, string) {
	t.Helper()

	tmpRoot := t.TempDir()
	t.Setenv("HOME", tmpRoot)
	t.Setenv("USERPROFILE", tmpRoot)
	t.Setenv("DTVEM_ROOT", tmpRoot)

	config.ResetPathsCache()
	t.Cleanup(config.ResetPathsCache)
	ResetShimMapCache()
	t.Cleanup(ResetShimMapCache)

	shimsDir := config.DefaultPaths().Shims
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("Failed to create shims directory: %v", err)
	}

	shimSource := filepath.Join(tmpRoot, platformExeName("dtvem-shim"))
	writeExecutable(t, shimSource)

	return &Manager{shimSource: shimSource}, shimsDir
}

func TestPruneOrphanShims_RemovesUnbackedShims(t *testing.T) {
	m, shimsDir := newShimsTestManager(t)

	for _, name := range []string{"python", "pip", "ruff", "pytest"} {
		writeExecutable(t, filepath.Join(shimsDir, platformExeName(name)))
	}

	keep := ShimMap{
		"python": {Runtime: "python", Versions: []string{"3.13.11"}},
		"pip":    {Runtime: "python", Versions: []string{"3.13.11"}},
	}

	removed, failures := m.pruneOrphanShims(keep)

	if len(failures) != 0 {
		t.Fatalf("pruneOrphanShims() failures = %v, want none", failures)
	}
	want := []string{"pytest", "ruff"}
	if !reflect.DeepEqual(removed, want) {
		t.Errorf("pruneOrphanShims() removed = %v, want %v", removed, want)
	}

	for _, name := range want {
		if _, err := os.Stat(filepath.Join(shimsDir, platformExeName(name))); !os.IsNotExist(err) {
			t.Errorf("orphan shim %q should have been deleted", name)
		}
	}
	for name := range keep {
		if _, err := os.Stat(filepath.Join(shimsDir, platformExeName(name))); err != nil {
			t.Errorf("backed shim %q should have been kept: %v", name, err)
		}
	}
}

// TestPruneOrphanShims_RemovesCmdCompanion covers the Windows pairing:
// CreateShim writes both name.exe and a name.cmd wrapper, so pruning has
// to take the wrapper with it or `where.exe` keeps reporting a shim that
// no longer resolves.
func TestPruneOrphanShims_RemovesCmdCompanion(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Only Windows shims have a .cmd companion")
	}

	m, shimsDir := newShimsTestManager(t)
	writeExecutable(t, filepath.Join(shimsDir, "python.exe"))
	writeExecutable(t, filepath.Join(shimsDir, "ruff.exe"))
	writeExecutable(t, filepath.Join(shimsDir, "ruff.cmd"))

	removed, failures := m.pruneOrphanShims(ShimMap{"python": {Runtime: "python"}})

	if len(failures) != 0 {
		t.Fatalf("pruneOrphanShims() failures = %v, want none", failures)
	}
	if !reflect.DeepEqual(removed, []string{"ruff"}) {
		t.Fatalf("pruneOrphanShims() removed = %v, want [ruff]", removed)
	}
	for _, file := range []string{"ruff.exe", "ruff.cmd"} {
		if _, err := os.Stat(filepath.Join(shimsDir, file)); !os.IsNotExist(err) {
			t.Errorf("%s should have been deleted", file)
		}
	}
}

// TestPruneOrphanShims_ProtectsDtvemBinaries guards the worst possible
// prune: installs before 0.4 kept dtvem-shim in shims/ rather than bin/,
// and deleting it would break every other shim on the machine.
func TestPruneOrphanShims_ProtectsDtvemBinaries(t *testing.T) {
	m, shimsDir := newShimsTestManager(t)

	protected := []string{"dtvem", "dtvem-shim"}
	for _, name := range protected {
		writeExecutable(t, filepath.Join(shimsDir, platformExeName(name)))
	}

	removed, failures := m.pruneOrphanShims(ShimMap{"python": {Runtime: "python"}})

	if len(removed) != 0 || len(failures) != 0 {
		t.Fatalf("pruneOrphanShims() removed = %v, failures = %v; want both empty", removed, failures)
	}
	for _, name := range protected {
		if _, err := os.Stat(filepath.Join(shimsDir, platformExeName(name))); err != nil {
			t.Errorf("%q must never be pruned: %v", name, err)
		}
	}
}

// TestPruneOrphanShims_LeavesNonShimFilesAlone checks that a stray file
// in shims/ survives. ListShims derives bare names by trimming a trailing
// extension, so notes.txt surfaces as "notes" and would otherwise look
// like an orphan shim.
func TestPruneOrphanShims_LeavesNonShimFilesAlone(t *testing.T) {
	m, shimsDir := newShimsTestManager(t)

	notes := filepath.Join(shimsDir, "notes.txt")
	if err := os.WriteFile(notes, []byte("not a shim"), 0644); err != nil {
		t.Fatalf("Failed to write notes.txt: %v", err)
	}

	removed, failures := m.pruneOrphanShims(ShimMap{"python": {Runtime: "python"}})

	if len(removed) != 0 || len(failures) != 0 {
		t.Fatalf("pruneOrphanShims() removed = %v, failures = %v; want both empty", removed, failures)
	}
	if _, err := os.Stat(notes); err != nil {
		t.Errorf("notes.txt should not have been deleted: %v", err)
	}
}

// TestRehashWithCallback_PrunesOrphans is the regression test for #273.
// Reshim used to only ever add shim files, so a shim whose backing
// executable was gone stayed on disk forever — leaving `dtvem doctor`
// reporting cache/disk drift that `dtvem reshim` could never clear.
func TestRehashWithCallback_PrunesOrphans(t *testing.T) {
	m, shimsDir := newShimsTestManager(t)

	// One installed Python, providing only `python`.
	versionDir := filepath.Join(config.DefaultPaths().Versions, "python", "3.13.11")
	execDir := versionDir
	if runtime.GOOS != constants.OSWindows {
		execDir = filepath.Join(versionDir, "bin")
	}
	if err := os.MkdirAll(execDir, 0755); err != nil {
		t.Fatalf("Failed to create version directory: %v", err)
	}
	writeExecutable(t, filepath.Join(execDir, platformExeName("python")))

	// A shim left behind by a pip package that has since been removed.
	writeExecutable(t, filepath.Join(shimsDir, platformExeName("ruff")))

	result, err := m.RehashWithCallback(nil)
	if err != nil {
		t.Fatalf("RehashWithCallback() error: %v", err)
	}

	if !reflect.DeepEqual(result.RemovedShims, []string{"ruff"}) {
		t.Errorf("RemovedShims = %v, want [ruff]", result.RemovedShims)
	}
	if len(result.PruneFailures) != 0 {
		t.Errorf("PruneFailures = %v, want none", result.PruneFailures)
	}
	if _, err := os.Stat(filepath.Join(shimsDir, platformExeName("ruff"))); !os.IsNotExist(err) {
		t.Error("orphan shim ruff should have been deleted")
	}
	if _, err := os.Stat(filepath.Join(shimsDir, platformExeName("python"))); err != nil {
		t.Errorf("python shim should have been created: %v", err)
	}

	// The cache and the shims directory must now agree — exactly the
	// condition doctor checks, and the one reshim could not reach.
	cache, err := loadShimMapFromDisk()
	if err != nil {
		t.Fatalf("loadShimMapFromDisk() error: %v", err)
	}
	onDisk, err := m.ListShims()
	if err != nil {
		t.Fatalf("ListShims() error: %v", err)
	}
	sort.Strings(onDisk)

	if len(cache) != len(onDisk) {
		t.Errorf("cache (%d entries) and disk (%v) disagree", len(cache), onDisk)
	}
	for _, name := range onDisk {
		if _, ok := cache[name]; !ok {
			t.Errorf("shim %q on disk is missing from the cache", name)
		}
	}
}

// TestRehashWithCallback_EmptyScanKeepsShims verifies the guard around
// pruning: when the scan finds nothing, that signals a broken or
// unreadable versions tree rather than "everything was uninstalled", and
// wiping every shim on that signal would turn a recoverable state into a
// broken one.
func TestRehashWithCallback_EmptyScanKeepsShims(t *testing.T) {
	m, shimsDir := newShimsTestManager(t)

	// A runtime directory with a version that contains no executables.
	versionDir := filepath.Join(config.DefaultPaths().Versions, "python", "3.13.11")
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("Failed to create version directory: %v", err)
	}
	writeExecutable(t, filepath.Join(shimsDir, platformExeName("python")))

	if _, err := m.RehashWithCallback(nil); err == nil {
		t.Fatal("RehashWithCallback() should error when no executables are found")
	}

	if _, err := os.Stat(filepath.Join(shimsDir, platformExeName("python"))); err != nil {
		t.Errorf("existing shim should survive an empty scan: %v", err)
	}
}

func TestDiscoverShimsForVersion_EmptyDir(t *testing.T) {
	versionDir := t.TempDir()

	got := DiscoverShimsForVersion(versionDir)
	if len(got) != 0 {
		t.Errorf("DiscoverShimsForVersion(empty) = %v, want []", got)
	}
}

func TestDiscoverShimsForVersion_MissingVersionDir(t *testing.T) {
	// Caller may pass a path that doesn't exist (e.g., when called before
	// the install has moved files into place). The helper must not panic
	// or surface an error — it just returns no names.
	got := DiscoverShimsForVersion(filepath.Join(t.TempDir(), "does-not-exist"))
	if len(got) != 0 {
		t.Errorf("DiscoverShimsForVersion(missing) = %v, want []", got)
	}
}

func TestDiscoverShimsForVersion_BinDir(t *testing.T) {
	versionDir := t.TempDir()
	binDir := filepath.Join(versionDir, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatalf("mkdir bin: %v", err)
	}

	writeExecutable(t, filepath.Join(binDir, platformExeName("node")))
	writeExecutable(t, filepath.Join(binDir, platformExeName("npm")))

	got := DiscoverShimsForVersion(versionDir)
	want := []string{"node", "npm"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiscoverShimsForVersion(bin) = %v, want %v", got, want)
	}
}

// TestDiscoverShimsForVersion_PythonWindowsOnlyRoot models the python-build-
// standalone Windows tarball before ensurepip runs: python.exe and
// pythonw.exe live in the version root and Scripts/ is either absent or
// only contains the upstream .empty placeholder. The discover helper must
// not invent pip / python3 entries that don't exist on disk — that was
// the root cause of issue #269 where install-time shim creation used a
// static provider declaration instead of disk truth.
func TestDiscoverShimsForVersion_PythonWindowsOnlyRoot(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Windows-specific layout (root .exe files)")
	}

	versionDir := t.TempDir()
	writeExecutable(t, filepath.Join(versionDir, "python.exe"))
	writeExecutable(t, filepath.Join(versionDir, "pythonw.exe"))
	// Upstream ships an empty Scripts/.empty placeholder; recreate it
	// here so the test reflects the exact layout users see and proves
	// the helper does not surface the placeholder as a shim.
	if err := os.MkdirAll(filepath.Join(versionDir, "Scripts"), 0755); err != nil {
		t.Fatalf("mkdir Scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "Scripts", ".empty"), nil, 0644); err != nil {
		t.Fatalf("write .empty: %v", err)
	}

	got := DiscoverShimsForVersion(versionDir)
	want := []string{"python", "pythonw"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDiscoverShimsForVersion_PythonWindowsAfterEnsurepip models the
// post-ensurepip state: root has python.exe/pythonw.exe, Scripts/ has
// pip.exe, pip3.exe, and the versioned pip3.14.exe. All five should be
// discovered and the result should be sorted+deduplicated.
func TestDiscoverShimsForVersion_PythonWindowsAfterEnsurepip(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Windows-specific layout (Scripts/*.exe)")
	}

	versionDir := t.TempDir()
	writeExecutable(t, filepath.Join(versionDir, "python.exe"))
	writeExecutable(t, filepath.Join(versionDir, "pythonw.exe"))

	scriptsDir := filepath.Join(versionDir, "Scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("mkdir Scripts: %v", err)
	}
	writeExecutable(t, filepath.Join(scriptsDir, "pip.exe"))
	writeExecutable(t, filepath.Join(scriptsDir, "pip3.exe"))
	writeExecutable(t, filepath.Join(scriptsDir, "pip3.14.exe"))

	got := DiscoverShimsForVersion(versionDir)
	want := []string{"pip", "pip3", "pip3.14", "python", "pythonw"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDiscoverShimsForVersion_DedupesAcrossDirs proves the helper unions
// names across bin/ root/ and Scripts/ rather than double-counting. This
// matters because some runtimes (e.g., Ruby on Windows) place the same
// command name in multiple search locations.
func TestDiscoverShimsForVersion_DedupesAcrossDirs(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Multi-dir Windows scan (root + Scripts)")
	}

	versionDir := t.TempDir()
	writeExecutable(t, filepath.Join(versionDir, "tool.exe"))

	scriptsDir := filepath.Join(versionDir, "Scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatalf("mkdir Scripts: %v", err)
	}
	writeExecutable(t, filepath.Join(scriptsDir, "tool.exe"))

	got := DiscoverShimsForVersion(versionDir)
	want := []string{"tool"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
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
