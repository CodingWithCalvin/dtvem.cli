package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// xdgInstall plants a dtvem-shaped versions tree under root so the
// rootHasData heuristic returns true. Tests use this to set up the
// "split state" condition without writing real runtime binaries.
func xdgInstall(t *testing.T, root string) {
	t.Helper()
	versionDir := filepath.Join(root, "versions", "python", "3.11.0")
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
}

func newXDGSplitCheckWith(active string, candidates []string) *xdgSplitCheck {
	c := newXDGSplitCheck()
	c.activeRoot = func() string { return active }
	c.candidateRoots = func() []string { return candidates }
	// Defer to the real hasData against the filesystem so tests
	// exercise the rootHasData heuristic too rather than mocking it
	// away.
	return c
}

func TestXDGSplitCheck_SingleInstallIsOK(t *testing.T) {
	root := t.TempDir()
	xdgInstall(t, root)
	got := newXDGSplitCheckWith(root, []string{root}).Run()
	if !got.OK {
		t.Errorf("expected OK with a single install location, got %#v", got)
	}
}

func TestXDGSplitCheck_OrphanInstallIsFlagged(t *testing.T) {
	active := t.TempDir()
	xdgInstall(t, active)
	orphan := t.TempDir()
	xdgInstall(t, orphan)

	got := newXDGSplitCheckWith(active, []string{active, orphan}).Run()
	if got.OK {
		t.Fatalf("expected non-OK with split state, got OK")
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity: got %s, want warning", got.Severity)
	}
	if got.Fixable() {
		t.Errorf("xdg-split check should be manual, but Finding.Fix is set")
	}

	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, orphan) {
		t.Errorf("expected orphan path %q in details, got %#v", orphan, got.Details)
	}
}

func TestXDGSplitCheck_EmptyCandidateDirIsIgnored(t *testing.T) {
	// A candidate root with no versions/<runtime>/<version> tree is
	// not "data" — that's a freshly-initialized but empty install.
	// We shouldn't flag it as orphaned.
	active := t.TempDir()
	xdgInstall(t, active)
	emptyOrphan := t.TempDir()
	// no xdgInstall call → no version subdirs

	got := newXDGSplitCheckWith(active, []string{active, emptyOrphan}).Run()
	if !got.OK {
		t.Errorf("expected OK when secondary location is empty, got %#v", got)
	}
}

func TestXDGSplitCheck_ActiveLocationDuplicateIsCollapsed(t *testing.T) {
	// candidateRoots may legitimately list the active root more than
	// once (e.g., DTVEM_ROOT and XDG_DATA_HOME pointing at the same
	// place). That shouldn't be flagged as a split.
	active := t.TempDir()
	xdgInstall(t, active)
	got := newXDGSplitCheckWith(active, []string{active, active, active}).Run()
	if !got.OK {
		t.Errorf("expected OK when candidates dedupe to active root, got %#v", got)
	}
}

func TestRootHasData_RequiresVersionDir(t *testing.T) {
	// Skeleton with no version subdirectory should NOT count as data.
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "versions", "python"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if rootHasData(root) {
		t.Errorf("rootHasData should return false when no version subdir exists")
	}

	// Adding a version subdir flips it.
	if err := os.MkdirAll(filepath.Join(root, "versions", "python", "3.11.0"), 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !rootHasData(root) {
		t.Errorf("rootHasData should return true once a version subdir exists")
	}
}

func TestRootHasData_MissingRootIsFalse(t *testing.T) {
	if rootHasData(filepath.Join(t.TempDir(), "does-not-exist")) {
		t.Errorf("rootHasData should return false for a missing root")
	}
}

func TestDedupePaths_RemovesExactDuplicates(t *testing.T) {
	in := []string{"/a", "/b", "/a", "/c"}
	got := dedupePaths(in)
	want := []string{"/a", "/b", "/c"}
	if !stringSliceEqualLocal(got, want) {
		t.Errorf("dedupePaths(%v) = %v, want %v", in, got, want)
	}
}

func TestXDGSplitCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "xdg-split-state" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("xdg-split-state check is not in the default registry")
	}
}

// stringSliceEqualLocal is a copy of the unexported helper used by
// other test files in this package. Inlined to avoid widening the
// public API surface of the package just for tests.
func stringSliceEqualLocal(a, b []string) bool {
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
