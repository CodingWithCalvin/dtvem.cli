package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
)

// fakeRehasher is a rehasher impl used to verify the Fix closure wires
// through to Rehash without standing up a real shim.Manager (which
// needs the dtvem-shim binary on disk).
type fakeRehasher struct {
	called bool
	err    error
}

func (f *fakeRehasher) Rehash() (*shim.RehashResult, error) {
	f.called = true
	if f.err != nil {
		return nil, f.err
	}
	return &shim.RehashResult{TotalShims: 1}, nil
}

// emptyShimsLayout produces a fresh versions/shims directory pair
// rooted at root and seeded with the requested counts. Each call to
// versionsDir() returns versions, and shimsDir() returns shims, so the
// check can be exercised against a clean filesystem state per test.
func emptyShimsLayout(t *testing.T, numRuntimes, numVersions, numShims int) (versions, shims string) {
	t.Helper()
	root := t.TempDir()
	versions = filepath.Join(root, "versions")
	shims = filepath.Join(root, "shims")

	if numRuntimes > 0 {
		if err := os.MkdirAll(versions, 0755); err != nil {
			t.Fatalf("mkdir versions: %v", err)
		}
		for i := range numRuntimes {
			runtimeDir := filepath.Join(versions, "runtime"+string(rune('a'+i)))
			if err := os.MkdirAll(runtimeDir, 0755); err != nil {
				t.Fatalf("mkdir runtime: %v", err)
			}
			for j := range numVersions {
				if err := os.MkdirAll(filepath.Join(runtimeDir, "v"+string(rune('0'+j))), 0755); err != nil {
					t.Fatalf("mkdir version: %v", err)
				}
			}
		}
	}

	if numShims > 0 {
		if err := os.MkdirAll(shims, 0755); err != nil {
			t.Fatalf("mkdir shims: %v", err)
		}
		for i := range numShims {
			f := filepath.Join(shims, "shim"+string(rune('a'+i)))
			if err := os.WriteFile(f, []byte{}, 0644); err != nil {
				t.Fatalf("write shim: %v", err)
			}
		}
	}

	return versions, shims
}

func newEmptyShimsCheckFor(versions, shims string, rh *fakeRehasher) *emptyShimsCheck {
	c := newEmptyShimsCheck()
	c.versionsDir = func() string { return versions }
	c.shimsDir = func() string { return shims }
	c.newManager = func() (rehasher, error) { return rh, nil }
	return c
}

func TestEmptyShimsCheck_NoRuntimesIsOK(t *testing.T) {
	// No runtimes installed → empty shims is expected, not a problem.
	versions, shims := emptyShimsLayout(t, 0, 0, 0)
	got := newEmptyShimsCheckFor(versions, shims, nil).Run()
	if !got.OK {
		t.Errorf("expected OK with no runtimes installed, got %#v", got)
	}
}

func TestEmptyShimsCheck_RuntimesAndShimsBothPresentIsOK(t *testing.T) {
	versions, shims := emptyShimsLayout(t, 1, 1, 3)
	got := newEmptyShimsCheckFor(versions, shims, nil).Run()
	if !got.OK {
		t.Errorf("expected OK with runtimes and shims present, got %#v", got)
	}
}

func TestEmptyShimsCheck_RuntimesPresentButShimsEmpty(t *testing.T) {
	versions, shims := emptyShimsLayout(t, 1, 1, 0)
	got := newEmptyShimsCheckFor(versions, shims, &fakeRehasher{}).Run()
	if got.OK {
		t.Fatalf("expected non-OK when runtimes exist but shims dir is empty, got OK")
	}
	if got.Severity != SeverityError {
		t.Errorf("severity: got %s, want error", got.Severity)
	}
	if !got.Fixable() {
		t.Errorf("empty-shims check should be fixable")
	}
}

func TestEmptyShimsCheck_FixInvokesRehash(t *testing.T) {
	versions, shims := emptyShimsLayout(t, 1, 1, 0)
	rh := &fakeRehasher{}
	got := newEmptyShimsCheckFor(versions, shims, rh).Run()
	if !got.Fixable() {
		t.Fatalf("precondition: expected fixable finding")
	}
	if err := got.Fix(); err != nil {
		t.Fatalf("Fix returned error: %v", err)
	}
	if !rh.called {
		t.Errorf("Fix did not invoke Rehash")
	}
}

func TestEmptyShimsCheck_FixPropagatesRehashError(t *testing.T) {
	versions, shims := emptyShimsLayout(t, 1, 1, 0)
	wantErr := errors.New("disk full")
	rh := &fakeRehasher{err: wantErr}
	got := newEmptyShimsCheckFor(versions, shims, rh).Run()
	err := got.Fix()
	if err == nil {
		t.Fatal("expected Fix to return an error")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Fix error: got %v, want chain containing %v", err, wantErr)
	}
}

func TestEmptyShimsCheck_RuntimeDirWithNoVersionsCountsAsZero(t *testing.T) {
	// A runtime directory with no version subdirectories means no
	// actual installs — even if there are several runtime folders, the
	// check should treat this as "nothing installed" rather than
	// flagging missing shims.
	versions, shims := emptyShimsLayout(t, 2, 0, 0)
	got := newEmptyShimsCheckFor(versions, shims, nil).Run()
	if !got.OK {
		t.Errorf("expected OK when no version subdirs exist, got %#v", got)
	}
}

func TestEmptyShimsCheck_MissingVersionsDirIsOK(t *testing.T) {
	// versions/ not existing yet is the freshly-initialized state.
	got := newEmptyShimsCheckFor("/definitely/does/not/exist/versions", "/definitely/does/not/exist/shims", nil).Run()
	if !got.OK {
		t.Errorf("expected OK when versions dir missing, got %#v", got)
	}
}

func TestEmptyShimsCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "empty-shims-directory" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("empty-shims-directory check is not in the default registry")
	}
}
