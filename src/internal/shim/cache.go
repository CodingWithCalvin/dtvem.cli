// Package shim manages shim executables that intercept runtime commands
package shim

import (
	"encoding/json"
	"os"
	"sync"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
)

// ShimMap represents the shim-to-runtime mapping cache
// The map key is the shim name (e.g., "tsc", "npm", "black")
// The map value is the runtime name (e.g., "node", "python")
type ShimMap map[string]string

var (
	shimMapCache     ShimMap
	shimMapCacheOnce sync.Once
	shimMapCacheErr  error
)

// LoadShimMap loads the shim-to-runtime mapping from the cache file.
// It uses sync.Once to ensure the cache is only loaded once per process.
// Returns the cached map and any error that occurred during loading.
func LoadShimMap() (ShimMap, error) {
	shimMapCacheOnce.Do(func() {
		shimMapCache, shimMapCacheErr = loadShimMapFromDisk()
	})
	return shimMapCache, shimMapCacheErr
}

// loadShimMapFromDisk reads the shim map cache file from disk
func loadShimMapFromDisk() (ShimMap, error) {
	cachePath := config.ShimMapPath()

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var shimMap ShimMap
	if err := json.Unmarshal(data, &shimMap); err != nil {
		return nil, err
	}

	return shimMap, nil
}

// SaveShimMap writes the shim-to-runtime mapping to the cache file.
// This should be called during reshim operations.
func SaveShimMap(shimMap ShimMap) error {
	// Ensure cache directory exists
	paths := config.DefaultPaths()
	if err := os.MkdirAll(paths.Cache, 0755); err != nil {
		return err
	}

	cachePath := config.ShimMapPath()

	data, err := json.MarshalIndent(shimMap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

// MergeShimMap merges the given entries into the on-disk shim map and persists it.
//
// If the cache does not exist yet (first-time install), a new map is created.
// Existing entries with matching keys are overwritten. The in-memory cache is
// reset so subsequent LoadShimMap calls read the updated state from disk.
//
// This is the preferred path for install-time shim registration, where the
// caller knows only the shims it just created and wants to register them
// without rebuilding the entire map (which would require scanning every
// installed runtime — `Rehash` does that).
func MergeShimMap(entries ShimMap) error {
	existing, err := loadShimMapFromDisk()
	if err != nil || existing == nil {
		// Cache missing, unreadable, or empty — start a fresh map.
		existing = make(ShimMap, len(entries))
	}

	for shim, runtime := range entries {
		existing[shim] = runtime
	}

	// Force the next LoadShimMap to re-read from disk so the merged entries
	// are visible to any subsequent caller in the same process.
	ResetShimMapCache()

	return SaveShimMap(existing)
}

// LookupRuntime looks up the runtime for a given shim name using the cache.
// Returns the runtime name and true if found, or empty string and false if not.
func LookupRuntime(shimName string) (string, bool) {
	shimMap, err := LoadShimMap()
	if err != nil {
		return "", false
	}

	runtime, ok := shimMap[shimName]
	return runtime, ok
}

// ResetShimMapCache resets the cached shim map, forcing a reload on next access.
// This is primarily useful for testing.
func ResetShimMapCache() {
	shimMapCacheOnce = sync.Once{}
	shimMapCache = nil
	shimMapCacheErr = nil
}
