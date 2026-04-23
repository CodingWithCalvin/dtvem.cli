package shim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
)

func TestSaveAndLoadShimMap(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Set DTVEM_ROOT to use our temp directory
	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	// Reset the paths cache to pick up new DTVEM_ROOT
	config.ResetPathsCache()
	defer config.ResetPathsCache()

	// Reset the shim map cache
	ResetShimMapCache()
	defer ResetShimMapCache()

	// Create the cache directory
	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create a test shim map
	testMap := ShimMap{
		"node":   "node",
		"npm":    "node",
		"npx":    "node",
		"tsc":    "node",
		"eslint": "node",
		"python": "python",
		"pip":    "python",
		"black":  "python",
	}

	// Save the map
	if err := SaveShimMap(testMap); err != nil {
		t.Fatalf("Failed to save shim map: %v", err)
	}

	// Verify the file was created
	cachePath := config.ShimMapPath()
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatalf("Shim map cache file was not created at %s", cachePath)
	}

	// Load the map
	loadedMap, err := LoadShimMap()
	if err != nil {
		t.Fatalf("Failed to load shim map: %v", err)
	}

	// Verify all entries
	for shimName, expectedRuntime := range testMap {
		if loadedRuntime, ok := loadedMap[shimName]; !ok {
			t.Errorf("Shim %q not found in loaded map", shimName)
		} else if loadedRuntime != expectedRuntime {
			t.Errorf("Shim %q: expected runtime %q, got %q", shimName, expectedRuntime, loadedRuntime)
		}
	}
}

func TestLookupRuntime(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Set DTVEM_ROOT to use our temp directory
	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	// Reset the paths cache to pick up new DTVEM_ROOT
	config.ResetPathsCache()
	defer config.ResetPathsCache()

	// Reset the shim map cache
	ResetShimMapCache()
	defer ResetShimMapCache()

	// Create the cache directory
	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create and save a test shim map
	testMap := ShimMap{
		"node":   "node",
		"npm":    "node",
		"tsc":    "node",
		"python": "python",
		"black":  "python",
	}

	if err := SaveShimMap(testMap); err != nil {
		t.Fatalf("Failed to save shim map: %v", err)
	}

	// Test lookups
	tests := []struct {
		shimName        string
		expectedRuntime string
		expectedFound   bool
	}{
		{"node", "node", true},
		{"npm", "node", true},
		{"tsc", "node", true},
		{"python", "python", true},
		{"black", "python", true},
		{"unknown", "", false},
		{"", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.shimName, func(t *testing.T) {
			runtime, found := LookupRuntime(tc.shimName)
			if found != tc.expectedFound {
				t.Errorf("LookupRuntime(%q): expected found=%v, got found=%v", tc.shimName, tc.expectedFound, found)
			}
			if runtime != tc.expectedRuntime {
				t.Errorf("LookupRuntime(%q): expected runtime=%q, got runtime=%q", tc.shimName, tc.expectedRuntime, runtime)
			}
		})
	}
}

func TestLookupRuntimeNoCacheFile(t *testing.T) {
	// Create a temporary directory for the test (empty, no cache file)
	tempDir := t.TempDir()

	// Set DTVEM_ROOT to use our temp directory
	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	// Reset the paths cache to pick up new DTVEM_ROOT
	config.ResetPathsCache()
	defer config.ResetPathsCache()

	// Reset the shim map cache
	ResetShimMapCache()
	defer ResetShimMapCache()

	// Lookup should return not found when cache doesn't exist
	runtime, found := LookupRuntime("node")
	if found {
		t.Errorf("LookupRuntime should return found=false when cache doesn't exist")
	}
	if runtime != "" {
		t.Errorf("LookupRuntime should return empty runtime when cache doesn't exist, got %q", runtime)
	}
}

func TestShimMapCacheOnlyLoadsOnce(t *testing.T) {
	// Create a temporary directory for the test
	tempDir := t.TempDir()

	// Set DTVEM_ROOT to use our temp directory
	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	// Reset the paths cache to pick up new DTVEM_ROOT
	config.ResetPathsCache()
	defer config.ResetPathsCache()

	// Reset the shim map cache
	ResetShimMapCache()
	defer ResetShimMapCache()

	// Create the cache directory
	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Create and save initial shim map
	initialMap := ShimMap{"node": "node"}
	if err := SaveShimMap(initialMap); err != nil {
		t.Fatalf("Failed to save initial shim map: %v", err)
	}

	// Load the map
	map1, err := LoadShimMap()
	if err != nil {
		t.Fatalf("Failed to load shim map: %v", err)
	}

	// Modify the file on disk
	modifiedMap := ShimMap{"node": "modified", "new": "entry"}
	if err := SaveShimMap(modifiedMap); err != nil {
		t.Fatalf("Failed to save modified shim map: %v", err)
	}

	// Load again - should return cached version (sync.Once)
	map2, err := LoadShimMap()
	if err != nil {
		t.Fatalf("Failed to load shim map second time: %v", err)
	}

	// Both should be the same (initial map, not modified)
	if map1["node"] != map2["node"] {
		t.Errorf("Cache should return same map: map1[node]=%q, map2[node]=%q", map1["node"], map2["node"])
	}

	// The modified entry should not be present (cache wasn't reloaded)
	if _, ok := map2["new"]; ok {
		t.Errorf("Cache should not have reloaded - 'new' entry should not exist")
	}
}

func TestMergeShimMap_CreatesWhenNoExistingCache(t *testing.T) {
	tempDir := t.TempDir()

	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	config.ResetPathsCache()
	defer config.ResetPathsCache()

	ResetShimMapCache()
	defer ResetShimMapCache()

	// No cache directory pre-existing — MergeShimMap must create it from scratch.
	entries := ShimMap{
		"node": "node",
		"npm":  "node",
		"npx":  "node",
	}

	if err := MergeShimMap(entries); err != nil {
		t.Fatalf("MergeShimMap returned error on fresh install: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap after MergeShimMap failed: %v", err)
	}

	if len(loaded) != len(entries) {
		t.Errorf("expected %d entries, got %d (%v)", len(entries), len(loaded), loaded)
	}
	for shim, runtime := range entries {
		if got := loaded[shim]; got != runtime {
			t.Errorf("entry %q: expected runtime %q, got %q", shim, runtime, got)
		}
	}
}

func TestMergeShimMap_MergesIntoExistingCache(t *testing.T) {
	tempDir := t.TempDir()

	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	config.ResetPathsCache()
	defer config.ResetPathsCache()

	ResetShimMapCache()
	defer ResetShimMapCache()

	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Seed an existing cache (simulates a prior install).
	initial := ShimMap{
		"python": "python",
		"pip":    "python",
	}
	if err := SaveShimMap(initial); err != nil {
		t.Fatalf("seed SaveShimMap failed: %v", err)
	}

	// Merge in a disjoint set of entries (simulates installing a second runtime).
	added := ShimMap{
		"node": "node",
		"npm":  "node",
	}
	if err := MergeShimMap(added); err != nil {
		t.Fatalf("MergeShimMap failed: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap failed: %v", err)
	}

	// All four entries should now be present.
	wantAll := ShimMap{
		"python": "python",
		"pip":    "python",
		"node":   "node",
		"npm":    "node",
	}
	for shim, runtime := range wantAll {
		if got := loaded[shim]; got != runtime {
			t.Errorf("entry %q: expected runtime %q, got %q", shim, runtime, got)
		}
	}
}

func TestMergeShimMap_OverwritesExistingKeys(t *testing.T) {
	tempDir := t.TempDir()

	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	config.ResetPathsCache()
	defer config.ResetPathsCache()

	ResetShimMapCache()
	defer ResetShimMapCache()

	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Seed with a stale mapping (e.g., a shim that was previously attributed
	// to the wrong runtime by some prior state).
	stale := ShimMap{"corepack": "wrong"}
	if err := SaveShimMap(stale); err != nil {
		t.Fatalf("seed SaveShimMap failed: %v", err)
	}

	// Merge should overwrite with the correct runtime.
	if err := MergeShimMap(ShimMap{"corepack": "node"}); err != nil {
		t.Fatalf("MergeShimMap failed: %v", err)
	}

	loaded, err := LoadShimMap()
	if err != nil {
		t.Fatalf("LoadShimMap failed: %v", err)
	}

	if got := loaded["corepack"]; got != "node" {
		t.Errorf("expected corepack remapped to node, got %q", got)
	}
}

func TestMergeShimMap_ResetsInMemoryCache(t *testing.T) {
	tempDir := t.TempDir()

	originalRoot := os.Getenv("DTVEM_ROOT")
	_ = os.Setenv("DTVEM_ROOT", tempDir)
	defer func() { _ = os.Setenv("DTVEM_ROOT", originalRoot) }()

	config.ResetPathsCache()
	defer config.ResetPathsCache()

	ResetShimMapCache()
	defer ResetShimMapCache()

	cacheDir := filepath.Join(tempDir, "cache")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache directory: %v", err)
	}

	// Prime the in-memory cache with an initial map.
	if err := SaveShimMap(ShimMap{"node": "node"}); err != nil {
		t.Fatalf("SaveShimMap failed: %v", err)
	}
	if _, err := LoadShimMap(); err != nil {
		t.Fatalf("initial LoadShimMap failed: %v", err)
	}

	// Without ResetShimMapCache, the next Load would return the cached copy.
	// MergeShimMap is supposed to reset it so callers see merged state.
	if err := MergeShimMap(ShimMap{"npm": "node"}); err != nil {
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
