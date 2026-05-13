package doctor

import (
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// withPathEnv sets $PATH for the duration of t and restores it after.
// Tests rely on this to construct a known PATH list without depending
// on whatever the developer's shell happens to be exporting.
func withPathEnv(t *testing.T, entries []string) {
	t.Helper()
	sep := ":"
	if goruntime.GOOS == constants.OSWindows {
		sep = ";"
	}
	original := os.Getenv("PATH")
	t.Cleanup(func() { _ = os.Setenv("PATH", original) })
	_ = os.Setenv("PATH", strings.Join(entries, sep))
}

// withDtvemRoot points DTVEM_ROOT at a temp dir so path.ShimsDir()
// resolves to a known, test-controlled location. Returns the resolved
// shims directory the test can compare against.
func withDtvemRoot(t *testing.T) (root, shims string) {
	t.Helper()
	root = t.TempDir()
	original := os.Getenv("DTVEM_ROOT")
	t.Cleanup(func() { _ = os.Setenv("DTVEM_ROOT", original) })
	_ = os.Setenv("DTVEM_ROOT", root)
	return root, filepath.Join(root, "shims")
}

func TestStaleShimsCheck_NoStaleEntries(t *testing.T) {
	_, shims := withDtvemRoot(t)
	withPathEnv(t, []string{shims, "/usr/local/bin"})

	got := newStaleShimsCheck().Run()
	if !got.OK {
		t.Errorf("expected OK finding with only current shims in PATH, got %#v", got)
	}
}

func TestStaleShimsCheck_DetectsStaleEntry(t *testing.T) {
	_, shims := withDtvemRoot(t)

	// Construct a path that matches the dtvem shims pattern but is
	// different from the current resolved shims dir.
	stale := filepath.Join(t.TempDir(), ".dtvem", "shims")
	withPathEnv(t, []string{stale, shims, "/usr/local/bin"})

	got := newStaleShimsCheck().Run()
	if got.OK {
		t.Fatalf("expected non-OK finding when PATH contains stale shims entry, got OK")
	}
	if got.Severity != SeverityError {
		t.Errorf("severity: got %s, want error", got.Severity)
	}
	if !got.Fixable() {
		t.Errorf("stale-shims check should be fixable (Finding.Fix should be set)")
	}

	// The stale path must appear in the details so the user can find it.
	foundStale := false
	for _, d := range got.Details {
		if d.Key == "Found" && d.Value == stale {
			foundStale = true
			break
		}
	}
	if !foundStale {
		t.Errorf("expected stale path %q in Details, got %#v", stale, got.Details)
	}
}

func TestStaleShimsCheck_MultipleStaleEntriesAreAllReported(t *testing.T) {
	_, shims := withDtvemRoot(t)
	stale1 := filepath.Join(t.TempDir(), ".dtvem", "shims")
	stale2 := filepath.Join(t.TempDir(), "dtvem", "shims")
	withPathEnv(t, []string{stale1, stale2, shims})

	got := newStaleShimsCheck().Run()
	if got.OK {
		t.Fatalf("expected non-OK finding, got OK")
	}

	// Both stale entries should be present in Details.
	values := make(map[string]bool)
	for _, d := range got.Details {
		if d.Key == "Found" {
			values[d.Value] = true
		}
	}
	if !values[stale1] || !values[stale2] {
		t.Errorf("expected both stale entries in details, got values=%v", values)
	}

	// Title should pluralize for multiple entries.
	if !strings.Contains(got.Title, "entries") {
		t.Errorf("title should pluralize for >1 stale entries, got %q", got.Title)
	}
}

func TestStaleShimsCheck_Registered(t *testing.T) {
	// The init() function should add the stale-shims check to the
	// package-level registry. We confirm by name rather than instance
	// so the check can be re-implemented without breaking this test.
	found := false
	for _, c := range All() {
		if c.Name() == "stale-shims-path" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("stale-shims-path check is not in the default registry")
	}
}

func TestPlural(t *testing.T) {
	if got := plural(1, "y", "ies"); got != "y" {
		t.Errorf("plural(1) = %q, want %q", got, "y")
	}
	if got := plural(2, "y", "ies"); got != "ies" {
		t.Errorf("plural(2) = %q, want %q", got, "ies")
	}
	if got := plural(0, "y", "ies"); got != "ies" {
		t.Errorf("plural(0) = %q, want %q", got, "ies")
	}
}

// staleShimsTestCheck returns a staleShimsCheck wired with all fields
// injected so each test can drive both platform branches deterministically
// without depending on host env vars or the real registry/filesystem.
func staleShimsTestCheck(stalePath, current string, removeFromPath, removeFromFile func(string, string) ([]string, error)) *staleShimsCheck {
	c := newStaleShimsCheck()
	c.pathEnv = func() string {
		// Use an OS-specific separator so SplitPath round-trips.
		sep := ":"
		if goruntimeIsWindows() {
			sep = ";"
		}
		return stalePath + sep + current
	}
	c.currentShimsDir = func() string { return current }
	if removeFromPath != nil {
		c.removeFromPath = func(s string) ([]string, error) { return removeFromPath(s, current) }
	}
	if removeFromFile != nil {
		c.removeFromFile = removeFromFile
	}
	c.detectShell = func() string { return "bash" }
	c.shellConfigFile = func(_ string) string { return "/tmp/test-config" }
	return c
}

// goruntimeIsWindows mirrors runtime.GOOS == "windows" without
// importing the alias in this test file.
func goruntimeIsWindows() bool {
	return os.PathSeparator == '\\'
}

func TestStaleShimsCheck_FixCallsPlatformImpl(t *testing.T) {
	// Build a layout with one stale path and one current entry.
	root := t.TempDir()
	current := filepath.Join(root, "shims")
	stale := filepath.Join(t.TempDir(), ".dtvem", "shims")

	var pathCalled, fileCalled bool
	pathFix := func(_, _ string) ([]string, error) {
		pathCalled = true
		return []string{stale}, nil
	}
	fileFix := func(_, _ string) ([]string, error) {
		fileCalled = true
		return []string{stale}, nil
	}

	c := staleShimsTestCheck(stale, current, pathFix, fileFix)
	got := c.Run()
	if !got.Fixable() {
		t.Fatalf("precondition: expected fixable finding")
	}
	if err := got.Fix(); err != nil {
		t.Fatalf("Fix returned error: %v", err)
	}

	if goruntimeIsWindows() {
		if !pathCalled {
			t.Errorf("Windows fix should call removeFromPath, but didn't")
		}
		if fileCalled {
			t.Errorf("Windows fix unexpectedly called removeFromFile")
		}
	} else {
		if !fileCalled {
			t.Errorf("Unix fix should call removeFromFile, but didn't")
		}
		if pathCalled {
			t.Errorf("Unix fix unexpectedly called removeFromPath")
		}
	}
}

func TestStaleShimsCheck_FixReportsWhenNothingRemoved(t *testing.T) {
	// If the impl reports zero removals, the Fix should surface that
	// as an error so the user understands the cleanup didn't happen —
	// rather than silently succeeding and leaving them wondering why
	// the warning persists.
	root := t.TempDir()
	current := filepath.Join(root, "shims")
	stale := filepath.Join(t.TempDir(), ".dtvem", "shims")

	zero := func(_, _ string) ([]string, error) { return nil, nil }
	c := staleShimsTestCheck(stale, current, zero, zero)
	got := c.Run()
	if !got.Fixable() {
		t.Fatalf("precondition: expected fixable finding")
	}
	err := got.Fix()
	if err == nil {
		t.Fatal("expected error when impl removed nothing")
	}
}

func TestStaleShimsCheck_FixPropagatesImplError(t *testing.T) {
	root := t.TempDir()
	current := filepath.Join(root, "shims")
	stale := filepath.Join(t.TempDir(), ".dtvem", "shims")

	wantErr := errors.New("simulated registry failure")
	bad := func(_, _ string) ([]string, error) { return nil, wantErr }
	c := staleShimsTestCheck(stale, current, bad, bad)
	got := c.Run()
	if err := got.Fix(); !errors.Is(err, wantErr) {
		t.Errorf("Fix err = %v, want chain containing %v", err, wantErr)
	}
}
