package shim

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// touch creates an empty file at path, making any missing parent directories.
// On Unix it sets the executable bit so callers can rely on the file behaving
// like a real binary for path-resolution tests.
func touch(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create %s: %v", path, err)
	}
	_ = f.Close()
	if runtime.GOOS != constants.OSWindows {
		if err := os.Chmod(path, 0755); err != nil {
			t.Fatalf("chmod %s: %v", path, err)
		}
	}
}

// runtimeBin returns the conventional name for a primary runtime binary on the
// current platform — e.g. "python.exe" on Windows, "python" on Unix.
func runtimeBin(name string) string {
	if runtime.GOOS == constants.OSWindows {
		return name + constants.ExtExe
	}
	return name
}

// secondaryBin returns the conventional name for a secondary executable on the
// current platform. The .ext argument is the Windows extension to use; on Unix
// the extension is dropped because Unix scripts are typically extensionless.
func secondaryBin(name, ext string) string {
	if runtime.GOOS == constants.OSWindows {
		return name + ext
	}
	return name
}

func TestFindSecondaryExecutable_FoundAlongsideRuntime(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, runtimeBin("python"))
	touch(t, runtimePath)
	secondary := filepath.Join(dir, secondaryBin("pip", constants.ExtExe))
	touch(t, secondary)

	got, err := FindSecondaryExecutable(runtimePath, "pip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != secondary {
		t.Errorf("got %q, want %q", got, secondary)
	}
}

func TestFindSecondaryExecutable_FoundInScriptsSubdir(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Scripts/ subdirectory layout is Windows-specific (Python on Windows)")
	}
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, runtimeBin("python"))
	touch(t, runtimePath)
	secondary := filepath.Join(dir, "Scripts", secondaryBin("uv", constants.ExtExe))
	touch(t, secondary)

	got, err := FindSecondaryExecutable(runtimePath, "uv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != secondary {
		t.Errorf("got %q, want %q", got, secondary)
	}
}

func TestFindSecondaryExecutable_FoundInParentScriptsSubdir(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Scripts/ subdirectory layout is Windows-specific")
	}
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	runtimePath := filepath.Join(binDir, runtimeBin("python"))
	touch(t, runtimePath)
	secondary := filepath.Join(root, "Scripts", secondaryBin("uv", constants.ExtExe))
	touch(t, secondary)

	got, err := FindSecondaryExecutable(runtimePath, "uv")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// filepath.Clean should have collapsed the "..". Compare on cleaned form.
	if filepath.Clean(got) != filepath.Clean(secondary) {
		t.Errorf("got %q, want %q", got, secondary)
	}
}

func TestFindSecondaryExecutable_PrefersCmdOverExeOnWindows(t *testing.T) {
	if runtime.GOOS != constants.OSWindows {
		t.Skip("Windows extension preference is Windows-specific")
	}
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, runtimeBin("node"))
	touch(t, runtimePath)
	cmdPath := filepath.Join(dir, "npm"+constants.ExtCmd)
	exePath := filepath.Join(dir, "npm"+constants.ExtExe)
	touch(t, cmdPath)
	touch(t, exePath)

	got, err := FindSecondaryExecutable(runtimePath, "npm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != cmdPath {
		t.Errorf("got %q, want %q (.cmd should be preferred)", got, cmdPath)
	}
}

func TestFindSecondaryExecutable_NotFoundReturnsError(t *testing.T) {
	dir := t.TempDir()
	runtimePath := filepath.Join(dir, runtimeBin("python"))
	touch(t, runtimePath)

	got, err := FindSecondaryExecutable(runtimePath, "uv")
	if err == nil {
		t.Fatalf("expected error, got nil and path %q", got)
	}
	if !errors.Is(err, ErrSecondaryExecutableNotFound) {
		t.Errorf("expected ErrSecondaryExecutableNotFound, got %v", err)
	}
	if got != "" {
		t.Errorf("expected empty path on error, got %q", got)
	}
}
