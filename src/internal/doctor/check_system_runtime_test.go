package doctor

import (
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// fakeProvider is a runtime.ShimProvider stand-in used by these tests
// to avoid pulling in the real node/python/ruby providers (which would
// transitively import HTTP fetchers, manifests, etc.).
type fakeProvider struct {
	name        string
	displayName string
	shims       []string
}

func (f *fakeProvider) Name() string                            { return f.name }
func (f *fakeProvider) DisplayName() string                     { return f.displayName }
func (f *fakeProvider) Shims() []string                         { return f.shims }
func (f *fakeProvider) ExecutablePath(string) (string, error)   { return "", nil }
func (f *fakeProvider) IsInstalled(string) (bool, error)        { return false, nil }
func (f *fakeProvider) ShouldReshimAfter(string, []string) bool { return false }
func (f *fakeProvider) GetEnvironment(string) (map[string]string, error) {
	return map[string]string{}, nil
}

// writeExecutable creates a file at dir/name with platform-appropriate
// suffix and execute permission. Returns the full path.
func writeExecutable(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	full := filepath.Join(dir, name)
	if goruntime.GOOS == constants.OSWindows {
		full = filepath.Join(dir, name+constants.ExtExe)
	}
	if err := os.WriteFile(full, []byte("stub"), 0755); err != nil {
		t.Fatalf("write %s: %v", full, err)
	}
	return full
}

// systemRuntimeTestCheck returns a check wired up to deterministic
// PATH entries and provider list.
func systemRuntimeTestCheck(pathEntries []string, shimsDir string, providers []runtime.ShimProvider) *systemRuntimeCheck {
	c := newSystemRuntimeCheck()
	sep := ":"
	if goruntime.GOOS == constants.OSWindows {
		sep = ";"
	}
	c.pathEnv = func() string { return strings.Join(pathEntries, sep) }
	c.shimsDir = func() string { return shimsDir }
	c.listProviders = func() []runtime.ShimProvider { return providers }
	return c
}

func TestSystemRuntimeCheck_NoProvidersIsOK(t *testing.T) {
	got := systemRuntimeTestCheck(nil, "/shims", nil).Run()
	if !got.OK {
		t.Errorf("expected OK with no providers, got %#v", got)
	}
}

func TestSystemRuntimeCheck_ShimsDirNotInPathIsOK(t *testing.T) {
	// When the shims dir isn't on PATH at all, this check defers to
	// shimsInPathCheck and reports OK rather than double-flagging.
	providers := []runtime.ShimProvider{
		&fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
	}
	got := systemRuntimeTestCheck([]string{"/usr/bin"}, "/shims", providers).Run()
	if !got.OK {
		t.Errorf("expected OK when shims dir absent, got %#v", got)
	}
}

func TestSystemRuntimeCheck_DetectsSystemPythonAhead(t *testing.T) {
	tmp := t.TempDir()
	systemDir := filepath.Join(tmp, "system")
	shimsDir := filepath.Join(tmp, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("mkdir shims: %v", err)
	}
	writeExecutable(t, systemDir, "python")

	providers := []runtime.ShimProvider{
		&fakeProvider{name: "python", displayName: "Python", shims: []string{"python", "pip"}},
	}
	got := systemRuntimeTestCheck([]string{systemDir, shimsDir}, shimsDir, providers).Run()
	if got.OK {
		t.Fatalf("expected non-OK when system python is ahead, got OK")
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity: got %s, want warning", got.Severity)
	}
	if got.Fixable() {
		t.Errorf("system-runtime check should be manual, but Finding.Fix is set")
	}

	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, systemDir) {
		t.Errorf("expected system dir %q in details, got %#v", systemDir, got.Details)
	}
	if !strings.Contains(combined, "python") {
		t.Errorf("expected matched shim 'python' in details, got %#v", got.Details)
	}
}

func TestSystemRuntimeCheck_NoConflictWhenSystemDirIsAfterShims(t *testing.T) {
	tmp := t.TempDir()
	systemDir := filepath.Join(tmp, "system")
	shimsDir := filepath.Join(tmp, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("mkdir shims: %v", err)
	}
	writeExecutable(t, systemDir, "python")

	providers := []runtime.ShimProvider{
		&fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
	}
	// shimsDir comes first → dtvem wins → no conflict.
	got := systemRuntimeTestCheck([]string{shimsDir, systemDir}, shimsDir, providers).Run()
	if !got.OK {
		t.Errorf("expected OK when shims dir precedes system dir, got %#v", got)
	}
}

func TestSystemRuntimeCheck_StopsAtFirstMatchingDirPerRuntime(t *testing.T) {
	tmp := t.TempDir()
	firstDir := filepath.Join(tmp, "first")
	secondDir := filepath.Join(tmp, "second")
	shimsDir := filepath.Join(tmp, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("mkdir shims: %v", err)
	}
	writeExecutable(t, firstDir, "python")
	writeExecutable(t, secondDir, "python")

	providers := []runtime.ShimProvider{
		&fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
	}
	got := systemRuntimeTestCheck([]string{firstDir, secondDir, shimsDir}, shimsDir, providers).Run()
	if got.OK {
		t.Fatal("expected non-OK")
	}
	// Only the first dir should appear in details, not the second.
	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, firstDir) {
		t.Errorf("first dir should be in details: %#v", got.Details)
	}
	if strings.Contains(combined, secondDir) {
		t.Errorf("second dir should NOT be in details (only first match per runtime): %#v", got.Details)
	}
}

func TestSystemRuntimeCheck_MultipleRuntimes(t *testing.T) {
	tmp := t.TempDir()
	systemDir := filepath.Join(tmp, "system")
	shimsDir := filepath.Join(tmp, "shims")
	if err := os.MkdirAll(shimsDir, 0755); err != nil {
		t.Fatalf("mkdir shims: %v", err)
	}
	writeExecutable(t, systemDir, "python")
	writeExecutable(t, systemDir, "node")

	providers := []runtime.ShimProvider{
		&fakeProvider{name: "python", displayName: "Python", shims: []string{"python"}},
		&fakeProvider{name: "node", displayName: "Node.js", shims: []string{"node"}},
	}
	got := systemRuntimeTestCheck([]string{systemDir, shimsDir}, shimsDir, providers).Run()
	if got.OK {
		t.Fatalf("expected non-OK with two conflicting runtimes")
	}
	if len(got.Details) != 2 {
		t.Errorf("expected 2 details (one per runtime), got %d: %#v", len(got.Details), got.Details)
	}
}

func TestSystemRuntimeCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "system-runtime-precedence" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("system-runtime-precedence check is not in the default registry")
	}
}
