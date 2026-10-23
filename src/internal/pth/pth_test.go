package pth

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}

func TestFind(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "python39._pth"), "python39.zip\n.\n")
	writeFile(t, filepath.Join(dir, "python.exe"), "")
	writeFile(t, filepath.Join(dir, "python38._pth.disabled"), "")
	if err := os.MkdirAll(filepath.Join(dir, "Lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "Lib", "nested._pth"), "")

	got, err := Find(dir)
	if err != nil {
		t.Fatalf("Find() error: %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "python39._pth" {
		t.Errorf("Find() = %v, want only python39._pth", got)
	}
}

func TestFind_FullBuild(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "python.exe"), "")

	got, err := Find(dir)
	if err != nil || len(got) != 0 {
		t.Errorf("Find(full build) = %v, %v; want none", got, err)
	}
}

func TestDisable(t *testing.T) {
	dir := t.TempDir()
	pthFile := filepath.Join(dir, "python39._pth")
	writeFile(t, pthFile, "python39.zip\n.\nimport site\n")

	disabled, err := Disable(dir)
	if err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if len(disabled) != 1 || disabled[0] != pthFile {
		t.Errorf("Disable() = %v, want [%s]", disabled, pthFile)
	}
	if _, err := os.Stat(pthFile); !os.IsNotExist(err) {
		t.Errorf("%s still exists after Disable()", pthFile)
	}
	if got := readFile(t, pthFile+DisabledSuffix); got != "python39.zip\n.\nimport site\n" {
		t.Errorf("disabled file content = %q, want the original", got)
	}

	// Nothing left to disable.
	again, err := Disable(dir)
	if err != nil || len(again) != 0 {
		t.Errorf("second Disable() = %v, %v; want nothing", again, err)
	}
}

func TestDisable_ReplacesOldDisabledCopy(t *testing.T) {
	dir := t.TempDir()
	pthFile := filepath.Join(dir, "python39._pth")
	writeFile(t, pthFile, "new")
	writeFile(t, pthFile+DisabledSuffix, "old")

	if _, err := Disable(dir); err != nil {
		t.Fatalf("Disable() error: %v", err)
	}
	if got := readFile(t, pthFile+DisabledSuffix); got != "new" {
		t.Errorf("disabled file content = %q, want %q", got, "new")
	}
}

func TestDisable_MissingDirectory(t *testing.T) {
	disabled, err := Disable(filepath.Join(t.TempDir(), "missing"))
	if err != nil || len(disabled) != 0 {
		t.Errorf("Disable(missing dir) = %v, %v; want nothing", disabled, err)
	}
}
