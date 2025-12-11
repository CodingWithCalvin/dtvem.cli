package shim

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dtvem/dtvem/src/internal/config"
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
