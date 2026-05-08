package shim

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
)

// withTempCache sets up DTVEM_ROOT to a temp dir, creates the cache dir,
// and resets the various caches so each test starts from a clean slate.
// The returned cleanup must be deferred by the caller.
func withTempCache(t *testing.T) (cacheDir string, cleanup func()) {
	t.Helper()
	tempDir := t.TempDir()

	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	config.ResetPathsCache()
	ResetShimMapCache()

	cacheDir = filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	cleanup = func() {
		_ = os.Setenv("DTVEM_ROOT", originalRoot)
		config.ResetPathsCache()
		ResetShimMapCache()
	}
	return cacheDir, cleanup
}

// entry is a small constructor for ShimEntry test data.
func entry(runtime string, versions ...string) ShimEntry {
	return ShimEntry{Runtime: runtime, Versions: versions}
}

func TestSaveAndLoadShimMap(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	testMap := ShimMap{
		"node":   entry("node", "22.0.0"),
		"npm":    entry("node", "22.0.0"),
		"python": entry("python", "3.11.0"),
		"pip":    entry("python", "3.11.0"),
	}

	if err := SaveShimMap(testMap); err != nil {
		t.Fatalf("Failed to save shim map: %v", err)
	}

	if _, err := os.Stat(config.ShimMapPath()); os.IsNotExist(err) {
		t.Fatalf("Shim map cache file was not created at %s", config.ShimMapPath())
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("Failed to load shim map: %v", err)
	}

	if !reflect.DeepEqual(loaded, testMap) {
		t.Errorf("loaded map does not match saved map\n got: %#v\nwant: %#v", loaded, testMap)
	}
}

func TestLookupRuntime(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	testMap := ShimMap{
		"node":   entry("node", "22.0.0"),
		"npm":    entry("node", "22.0.0"),
		"python": entry("python", "3.11.0"),
	}
	if err := SaveShimMap(testMap); err != nil {
		t.Fatalf("Failed to save shim map: %v", err)
	}

	tests := []struct {
		shimName        string
		expectedRuntime string
		expectedFound   bool
	}{
		{"node", "node", true},
		{"npm", "node", true},
		{"python", "python", true},
		{"unknown", "", false},
		{"", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.shimName, func(t *testing.T) {
			runtime, found := LookupRuntime(tc.shimName)
			if found != tc.expectedFound {
				t.Errorf("LookupRuntime(%q): expected found=%v, got %v", tc.shimName, tc.expectedFound, found)
			}
			if runtime != tc.expectedRuntime {
				t.Errorf("LookupRuntime(%q): expected runtime=%q, got %q", tc.shimName, tc.expectedRuntime, runtime)
			}
		})
	}
}

func TestLookup_ReturnsFullEntry(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	testMap := ShimMap{
		"uv": entry("python", "3.9.9", "3.10.0"),
	}
	if err := SaveShimMap(testMap); err != nil {
		t.Fatalf("Failed to save shim map: %v", err)
	}

	got, ok := Lookup("uv")
	if !ok {
		t.Fatalf("Lookup(\"uv\") not found")
	}
	if got.Runtime != "python" {
		t.Errorf("Runtime: got %q, want %q", got.Runtime, "python")
	}
	if !reflect.DeepEqual(got.Versions, []string{"3.9.9", "3.10.0"}) {
		t.Errorf("Versions: got %v, want %v", got.Versions, []string{"3.9.9", "3.10.0"})
	}
}

func TestLookupNoCacheFile(t *testing.T) {
	tempDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()
	config.ResetPathsCache()
	defer config.ResetPathsCache()
	ResetShimMapCache()
	defer ResetShimMapCache()

	runtime, found := LookupRuntime("node")
	if found {
		t.Errorf("LookupRuntime should return found=false when cache doesn't exist")
	}
	if runtime != "" {
		t.Errorf("LookupRuntime should return empty runtime when cache doesn't exist, got %q", runtime)
	}
}

func TestLoadShimMap_LegacySchemaFallback(t *testing.T) {
	cacheDir, cleanup := withTempCache(t)
	defer cleanup()

	// Hand-write a legacy-schema cache file: shim → runtime name (string),
	// no version coverage data. Loaders must tolerate it so users who
	// upgrade dtvem don't see broken shims until they next run reshim.
	legacy := []byte(`{
  "uv": "python",
  "npm": "node"
}`)
	cachePath := filepath.Join(cacheDir, "shim-map.json")
	if err := os.WriteFile(cachePath, legacy, 0644); err != nil {
		t.Fatalf("write legacy cache: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap legacy: %v", err)
	}

	if got := loaded["uv"].Runtime; got != "python" {
		t.Errorf("uv runtime: got %q, want python", got)
	}
	if got := loaded["npm"].Runtime; got != "node" {
		t.Errorf("npm runtime: got %q, want node", got)
	}
	// Legacy entries should have no version coverage data — that's the
	// signal callers use to skip the version check.
	if got := loaded["uv"].Versions; len(got) != 0 {
		t.Errorf("uv versions on legacy load: got %v, want empty", got)
	}
}

func TestShimMapCacheOnlyLoadsOnce(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	if err := SaveShimMap(ShimMap{"node": entry("node", "22.0.0")}); err != nil {
		t.Fatalf("Failed to save initial shim map: %v", err)
	}

	first, err := LoadShimMap()
	if err != nil {
		t.Fatalf("Failed to load shim map: %v", err)
	}

	// Mutate on disk; LoadShimMap should still return the originally cached map.
	if err := SaveShimMap(ShimMap{"node": entry("modified"), "new": entry("entry")}); err != nil {
		t.Fatalf("Failed to save modified shim map: %v", err)
	}

	second, err := LoadShimMap()
	if err != nil {
		t.Fatalf("Failed to load shim map second time: %v", err)
	}

	if first["node"].Runtime != second["node"].Runtime {
		t.Errorf("cache should return same map: first=%q second=%q",
			first["node"].Runtime, second["node"].Runtime)
	}
	if _, ok := second["new"]; ok {
		t.Errorf("cache should not have reloaded - 'new' entry should not exist")
	}
}

func TestMergeShimMap_CreatesWhenNoExistingCache(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	// Wipe the cache directory to simulate a truly fresh install.
	_ = os.RemoveAll(filepath.Dir(config.ShimMapPath()))

	entries := ShimMap{
		"node": entry("node", "22.0.0"),
		"npm":  entry("node", "22.0.0"),
	}
	if err := MergeShimMap(entries); err != nil {
		t.Fatalf("MergeShimMap returned error on fresh install: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap after MergeShimMap failed: %v", err)
	}
	if !reflect.DeepEqual(loaded, entries) {
		t.Errorf("loaded mismatch\n got: %#v\nwant: %#v", loaded, entries)
	}
}

func TestMergeShimMap_MergesIntoExistingCache(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	// Seed an existing cache (simulates a prior install).
	initial := ShimMap{
		"python": entry("python", "3.11.0"),
		"pip":    entry("python", "3.11.0"),
	}
	if err := SaveShimMap(initial); err != nil {
		t.Fatalf("seed SaveShimMap failed: %v", err)
	}

	// Merge in disjoint entries (simulates installing a second runtime).
	added := ShimMap{
		"node": entry("node", "22.0.0"),
		"npm":  entry("node", "22.0.0"),
	}
	if err := MergeShimMap(added); err != nil {
		t.Fatalf("MergeShimMap failed: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap failed: %v", err)
	}

	want := ShimMap{
		"python": entry("python", "3.11.0"),
		"pip":    entry("python", "3.11.0"),
		"node":   entry("node", "22.0.0"),
		"npm":    entry("node", "22.0.0"),
	}
	if !reflect.DeepEqual(loaded, want) {
		t.Errorf("merged mismatch\n got: %#v\nwant: %#v", loaded, want)
	}
}

func TestMergeShimMap_UnionsVersionsForSameShim(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	// First install records version 3.9.9 for python's "uv" shim.
	if err := MergeShimMap(ShimMap{"uv": entry("python", "3.9.9")}); err != nil {
		t.Fatalf("first MergeShimMap failed: %v", err)
	}

	// Second install records version 3.10.0 for the same "uv" shim. The
	// cache must end up with both versions, not just the latest.
	if err := MergeShimMap(ShimMap{"uv": entry("python", "3.10.0")}); err != nil {
		t.Fatalf("second MergeShimMap failed: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap failed: %v", err)
	}

	got := loaded["uv"].Versions
	sort.Strings(got)
	want := []string{"3.10.0", "3.9.9"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("versions union: got %v, want %v", got, want)
	}
}

func TestMergeShimMap_OverwritesRuntimeName(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	// Seed with a stale mapping (e.g., a shim previously misattributed).
	if err := SaveShimMap(ShimMap{"corepack": entry("wrong", "1.0.0")}); err != nil {
		t.Fatalf("seed SaveShimMap failed: %v", err)
	}

	// A subsequent install with the correct attribution should overwrite
	// the runtime name.
	if err := MergeShimMap(ShimMap{"corepack": entry("node", "22.0.0")}); err != nil {
		t.Fatalf("MergeShimMap failed: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap failed: %v", err)
	}

	if got := loaded["corepack"].Runtime; got != "node" {
		t.Errorf("expected corepack remapped to node, got %q", got)
	}
}

func TestMergeShimMap_ResetsInMemoryCache(t *testing.T) {
	_, cleanup := withTempCache(t)
	defer cleanup()

	if err := SaveShimMap(ShimMap{"node": entry("node", "22.0.0")}); err != nil {
		t.Fatalf("SaveShimMap failed: %v", err)
	}
	if _, err := LoadShimMap(); err != nil {
		t.Fatalf("initial LoadShimMap failed: %v", err)
	}

	// MergeShimMap must invalidate the in-memory cache so the merged entry
	// is visible to the next caller in the same process.
	if err := MergeShimMap(ShimMap{"npm": entry("node", "22.0.0")}); err != nil {
		t.Fatalf("MergeShimMap failed: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("post-merge LoadShimMap failed: %v", err)
	}
	if _, ok := loaded["npm"]; !ok {
		t.Error("expected in-memory cache to be reset so the merged 'npm' entry is visible")
	}
}

func TestSaveShimMap_OmitsEmptyVersions(t *testing.T) {
	cacheDir, cleanup := withTempCache(t)
	defer cleanup()

	// An entry with no Versions should serialize without the field, so
	// that legacy-style or version-less records stay compact.
	if err := SaveShimMap(ShimMap{"uv": {Runtime: "python"}}); err != nil {
		t.Fatalf("SaveShimMap failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(cacheDir, "shim-map.json"))
	if err != nil {
		t.Fatalf("read cache: %v", err)
	}

	var parsed map[string]map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, data)
	}
	if _, hasVersions := parsed["uv"]["versions"]; hasVersions {
		t.Errorf("expected versions field to be omitted when empty, got: %s", data)
	}
}
