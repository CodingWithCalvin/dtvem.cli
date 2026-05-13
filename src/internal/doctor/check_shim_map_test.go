package doctor

import (
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
)

// shimFileName produces the on-disk filename for a shim, accounting
// for Windows requiring an .exe suffix. Mirrors what shim.Manager
// writes when it creates a shim.
func shimFileName(name string) string {
	if goruntime.GOOS == constants.OSWindows {
		return name + constants.ExtExe
	}
	return name
}

// shimMapLayout creates a temp shims directory pre-populated with the
// given shim names. Returns the directory path so a check can be
// pointed at it.
func shimMapLayout(t *testing.T, names ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		f := filepath.Join(dir, shimFileName(n))
		if err := os.WriteFile(f, []byte{}, 0644); err != nil {
			t.Fatalf("write shim %q: %v", n, err)
		}
	}
	return dir
}

// newShimMapCheckWith returns a check wired up to the provided shims
// directory and cache contents. Pass cacheErr to simulate a load
// failure that isn't a "missing file". The cache pointer can be
// reused so a test simulates rehash mutating the cache by writing
// into *cacheAfterFix from the rehasher closure.
func newShimMapCheckWith(shimsDir string, cache shim.ShimMap, cacheErr error, rh *fakeRehasher) *shimMapCheck {
	c := newShimMapCheck()
	c.shimsDir = func() string { return shimsDir }
	c.loadShimMap = func() (shim.ShimMap, error) { return cache, cacheErr }
	c.newManager = func() (rehasher, error) {
		if rh == nil {
			return nil, errors.New("no rehasher provided in test")
		}
		return rh, nil
	}
	return c
}

func TestShimMapCheck_BothEmptyIsOK(t *testing.T) {
	dir := shimMapLayout(t)
	got := newShimMapCheckWith(dir, shim.ShimMap{}, nil, nil).Run()
	if !got.OK {
		t.Errorf("expected OK with empty disk and empty cache, got %#v", got)
	}
}

func TestShimMapCheck_MatchingDiskAndCacheIsOK(t *testing.T) {
	dir := shimMapLayout(t, "python", "node")
	cache := shim.ShimMap{
		"python": {Runtime: "python"},
		"node":   {Runtime: "node"},
	}
	got := newShimMapCheckWith(dir, cache, nil, nil).Run()
	if !got.OK {
		t.Errorf("expected OK with matching disk and cache, got %#v", got)
	}
}

func TestShimMapCheck_DriftWhenCacheMissingEntries(t *testing.T) {
	dir := shimMapLayout(t, "python", "node", "ruby")
	cache := shim.ShimMap{
		"python": {Runtime: "python"},
		// "node" and "ruby" are on disk but not in the cache.
	}
	rh := &fakeRehasher{}
	got := newShimMapCheckWith(dir, cache, nil, rh).Run()
	if got.OK {
		t.Fatalf("expected non-OK when cache is missing entries, got OK")
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity: got %s, want warning", got.Severity)
	}
	if !got.Fixable() {
		t.Errorf("shim-map check should be fixable")
	}

	// The missing-in-cache names must appear in the details.
	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, "node") || !strings.Contains(combined, "ruby") {
		t.Errorf("expected node and ruby in details, got %#v", got.Details)
	}
}

func TestShimMapCheck_DriftWhenCacheHasGhostEntries(t *testing.T) {
	dir := shimMapLayout(t, "python")
	cache := shim.ShimMap{
		"python":  {Runtime: "python"},
		"deleted": {Runtime: "python"},
	}
	rh := &fakeRehasher{}
	got := newShimMapCheckWith(dir, cache, nil, rh).Run()
	if got.OK {
		t.Fatalf("expected non-OK when cache has entries with no disk file, got OK")
	}
	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, "deleted") {
		t.Errorf("expected ghost cache entry 'deleted' in details, got %#v", got.Details)
	}
}

func TestShimMapCheck_FixInvokesRehash(t *testing.T) {
	// Disk and cache stay in sync after the fake rehasher runs because
	// we point loadShimMap at a cache that already matches the disk
	// state — the rehasher itself doesn't write anything in tests.
	// This isolates the "did Fix actually call Rehash?" assertion
	// from the post-fix drift check.
	dir := shimMapLayout(t, "python")
	cacheBeforeFix := shim.ShimMap{}
	cacheAfterFix := shim.ShimMap{"python": {Runtime: "python"}}

	c := newShimMapCheck()
	c.shimsDir = func() string { return dir }
	loadCount := 0
	c.loadShimMap = func() (shim.ShimMap, error) {
		loadCount++
		if loadCount == 1 {
			return cacheBeforeFix, nil
		}
		return cacheAfterFix, nil
	}
	rh := &fakeRehasher{}
	c.newManager = func() (rehasher, error) { return rh, nil }

	got := c.Run()
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

func TestShimMapCheck_FixReportsResidualDrift(t *testing.T) {
	// Some drift can't be resolved by rehash alone — orphan shim
	// files persist because rehash doesn't delete shims whose
	// underlying runtime version doesn't actually provide that
	// executable. Verify the post-fix check surfaces the residual
	// drift as an error so the user doesn't see a misleading
	// "Fixed" green checkmark followed by the same warning on
	// the next run.
	dir := shimMapLayout(t, "orphan")
	emptyCache := shim.ShimMap{}

	c := newShimMapCheck()
	c.shimsDir = func() string { return dir }
	c.loadShimMap = func() (shim.ShimMap, error) { return emptyCache, nil }
	c.newManager = func() (rehasher, error) { return &fakeRehasher{}, nil }

	got := c.Run()
	if !got.Fixable() {
		t.Fatalf("precondition: expected fixable finding")
	}
	err := got.Fix()
	if err == nil {
		t.Fatal("expected Fix to error when residual drift remains")
	}
	if !strings.Contains(err.Error(), "orphan") {
		t.Errorf("expected error to mention orphan shims, got %q", err.Error())
	}
}

func TestShimMapCheck_CacheReadErrorIsWarning(t *testing.T) {
	dir := shimMapLayout(t, "python")
	got := newShimMapCheckWith(dir, nil, errors.New("corrupt cache"), nil).Run()
	if got.OK {
		t.Fatalf("expected non-OK when cache read fails, got OK")
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity: got %s, want warning", got.Severity)
	}
}

func TestShimMapCheck_MissingCacheFileIsTreatedAsEmpty(t *testing.T) {
	// os.IsNotExist errors from LoadShimMap should be treated as a
	// present-but-empty cache, which means an actual drift report
	// when there are shim files on disk — not a separate warning.
	dir := shimMapLayout(t, "python")
	missing := &os.PathError{Op: "open", Path: "x", Err: os.ErrNotExist}
	rh := &fakeRehasher{}
	got := newShimMapCheckWith(dir, nil, missing, rh).Run()
	if got.OK {
		t.Fatalf("expected non-OK when cache is missing and shims exist, got OK")
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity: got %s, want warning", got.Severity)
	}
}

func TestListShimNamesOnDisk_StripsExtensionsAndSkipsWrappers(t *testing.T) {
	dir := t.TempDir()
	// Create a .exe + .cmd pair (Windows install layout); both should
	// resolve to the same logical name "python", and the function
	// should not return "python" twice.
	if goruntime.GOOS == constants.OSWindows {
		if err := os.WriteFile(filepath.Join(dir, "python.exe"), []byte{}, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "python.cmd"), []byte{}, 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	} else {
		if err := os.WriteFile(filepath.Join(dir, "python"), []byte{}, 0755); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	got, err := listShimNamesOnDisk(dir)
	if err != nil {
		t.Fatalf("listShimNamesOnDisk: %v", err)
	}
	if len(got) != 1 || got[0] != "python" {
		t.Errorf("got %v, want [python]", got)
	}
}

func TestListShimNamesOnDisk_MissingDirIsEmpty(t *testing.T) {
	got, err := listShimNamesOnDisk(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Errorf("expected nil err for missing dir, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestSummarizeNames_Truncates(t *testing.T) {
	long := []string{"a", "b", "c", "d", "e", "f", "g"}
	got := summarizeNames(long)
	if !strings.Contains(got, "+2 more") {
		t.Errorf("expected summary to elide 2 names, got %q", got)
	}
}

func TestShimMapCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "shim-map-cache" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("shim-map-cache check is not in the default registry")
	}
}

// detailValues extracts the string values from a slice of Details for
// easier substring assertions in the tests above.
func detailValues(ds []Detail) []string {
	out := make([]string, len(ds))
	for i, d := range ds {
		out[i] = d.Value
	}
	return out
}
