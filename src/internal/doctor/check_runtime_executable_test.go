package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// execAwareProvider extends fakeProvider with a configurable
// ExecutablePath result so tests can plant working and broken
// installs side by side.
type execAwareProvider struct {
	*fakeProvider
	execPaths map[string]string
	execErr   error
}

func (p *execAwareProvider) ExecutablePath(version string) (string, error) {
	if p.execErr != nil {
		return "", p.execErr
	}
	return p.execPaths[version], nil
}

// installVersionDir creates versions/<runtime>/<version>/ under root
// and returns the runtime directory's full path so the test can plant
// an executable inside it.
func installVersionDir(t *testing.T, root, runtime, version string) string {
	t.Helper()
	dir := filepath.Join(root, "versions", runtime, version)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	return dir
}

func runtimeExecCheckWith(versionsDir string, providers map[string]*execAwareProvider) *runtimeExecutableCheck {
	c := newRuntimeExecutableCheck()
	c.versionsDir = func() string { return versionsDir }
	c.getProvider = func(name string) (runtime.ShimProvider, error) {
		p, ok := providers[name]
		if !ok {
			return nil, errors.New("provider not registered: " + name)
		}
		return p, nil
	}
	return c
}

func TestRuntimeExecutableCheck_NoInstallsIsOK(t *testing.T) {
	root := t.TempDir()
	got := runtimeExecCheckWith(filepath.Join(root, "versions"), nil).Run()
	if !got.OK {
		t.Errorf("expected OK with no installs, got %#v", got)
	}
}

func TestRuntimeExecutableCheck_HealthyInstallIsOK(t *testing.T) {
	root := t.TempDir()
	versionDir := installVersionDir(t, root, "python", "3.11.0")
	execPath := filepath.Join(versionDir, "bin", "python")
	if err := os.MkdirAll(filepath.Dir(execPath), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(execPath, []byte("stub"), 0755); err != nil {
		t.Fatalf("write: %v", err)
	}

	providers := map[string]*execAwareProvider{
		"python": {
			fakeProvider: &fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
			execPaths:    map[string]string{"3.11.0": execPath},
		},
	}
	got := runtimeExecCheckWith(filepath.Join(root, "versions"), providers).Run()
	if !got.OK {
		t.Errorf("expected OK with healthy install, got %#v", got)
	}
}

func TestRuntimeExecutableCheck_MissingExecutableIsError(t *testing.T) {
	root := t.TempDir()
	installVersionDir(t, root, "python", "3.11.0")
	// No file at execPath — install is incomplete.
	execPath := filepath.Join(root, "versions", "python", "3.11.0", "bin", "python")

	providers := map[string]*execAwareProvider{
		"python": {
			fakeProvider: &fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
			execPaths:    map[string]string{"3.11.0": execPath},
		},
	}
	got := runtimeExecCheckWith(filepath.Join(root, "versions"), providers).Run()
	if got.OK {
		t.Fatalf("expected non-OK with missing executable, got OK")
	}
	if got.Severity != SeverityError {
		t.Errorf("severity: got %s, want error", got.Severity)
	}
	if got.Fixable() {
		t.Errorf("runtime-executable check should be manual")
	}
	if !strings.Contains(got.Resolution, "dtvem install") {
		t.Errorf("resolution should suggest reinstall, got %q", got.Resolution)
	}
}

func TestRuntimeExecutableCheck_OrphanedRuntimeDirIsFlagged(t *testing.T) {
	root := t.TempDir()
	installVersionDir(t, root, "madeup", "1.0.0")
	got := runtimeExecCheckWith(filepath.Join(root, "versions"), nil).Run()
	if got.OK {
		t.Fatalf("expected non-OK when no provider matches install dir, got OK")
	}
	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, "no provider") && !strings.Contains(combined, "orphan") {
		t.Errorf("expected detail to mention missing provider or orphan, got %#v", got.Details)
	}
}

func TestRuntimeExecutableCheck_DirectoryAtExecutablePathIsFlagged(t *testing.T) {
	root := t.TempDir()
	versionDir := installVersionDir(t, root, "python", "3.11.0")
	execPath := filepath.Join(versionDir, "bin", "python")
	// Plant a directory where the executable should be — corrupt
	// state that's worth surfacing.
	if err := os.MkdirAll(execPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	providers := map[string]*execAwareProvider{
		"python": {
			fakeProvider: &fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
			execPaths:    map[string]string{"3.11.0": execPath},
		},
	}
	got := runtimeExecCheckWith(filepath.Join(root, "versions"), providers).Run()
	if got.OK {
		t.Fatalf("expected non-OK when a directory occupies the executable path")
	}
}

func TestRuntimeExecutableCheck_ProviderErrorIsFlagged(t *testing.T) {
	root := t.TempDir()
	installVersionDir(t, root, "python", "3.11.0")

	providers := map[string]*execAwareProvider{
		"python": {
			fakeProvider: &fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
			execErr:      errors.New("internal provider failure"),
		},
	}
	got := runtimeExecCheckWith(filepath.Join(root, "versions"), providers).Run()
	if got.OK {
		t.Fatalf("expected non-OK when provider errors, got OK")
	}
	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, "internal provider failure") {
		t.Errorf("expected provider error in details, got %#v", got.Details)
	}
}

func TestListInstalledVersions_MissingDirIsEmpty(t *testing.T) {
	got, err := listInstalledVersions(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Errorf("expected nil err for missing dir, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result, got %v", got)
	}
}

func TestListInstalledVersions_SortsByRuntimeThenVersion(t *testing.T) {
	root := t.TempDir()
	installVersionDir(t, root, "zruntime", "1.0.0")
	installVersionDir(t, root, "aruntime", "9.9.9")
	installVersionDir(t, root, "aruntime", "1.0.0")

	got, err := listInstalledVersions(filepath.Join(root, "versions"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	want := []installedVersion{
		{runtimeName: "aruntime", version: "1.0.0"},
		{runtimeName: "aruntime", version: "9.9.9"},
		{runtimeName: "zruntime", version: "1.0.0"},
	}
	if len(got) != len(want) {
		t.Fatalf("len mismatch: %v vs %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("index %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRuntimeExecutableCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "runtime-executable-present" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("runtime-executable-present check is not in the default registry")
	}
}
