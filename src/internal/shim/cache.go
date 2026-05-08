// Package shim manages shim executables that intercept runtime commands
package shim

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
)

// ShimEntry is the per-shim record stored in the shim-map cache. It binds
// a shim name to the runtime that owns it AND to the set of installed
// runtime versions that actually provide the executable on disk.
//
// The version data lets the shim distinguish between "command shimmed for
// this runtime, also present in the active version" and "command shimmed
// for this runtime, but the active version doesn't have it" — for example
// when `uv` is installed via `pip install uv` against Python 3.9 but the
// user switches to a 3.8 install that never received uv.
type ShimEntry struct {
	// Runtime is the runtime name (e.g., "python", "node", "ruby").
	Runtime string `json:"runtime"`

	// Versions is the set of installed runtime versions that provide the
	// executable. Empty / nil means "version coverage unknown" — typically
	// because the cache was loaded from the legacy schema or because the
	// caller did not supply version data — and callers should treat empty
	// as "skip the version check" rather than "no providing versions".
	Versions []string `json:"versions,omitempty"`
}

// ShimMap represents the shim-to-runtime mapping cache. The map key is the
// shim name (e.g., "tsc", "npm", "uv"); the value is a ShimEntry binding
// it to a runtime and (optionally) the set of versions that provide it.
type ShimMap map[string]ShimEntry

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

// loadShimMapFromDisk reads the shim map cache file from disk, tolerating
// both the current schema (ShimEntry values with versions) and the legacy
// flat schema (string values mapping shim → runtime). Legacy entries are
// converted with empty Versions, signaling "version coverage unknown" to
// callers; a subsequent `dtvem reshim` will re-populate the version data.
func loadShimMapFromDisk() (ShimMap, error) {
	cachePath := config.ShimMapPath()

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	// Try the current schema first.
	var current ShimMap
	if err := json.Unmarshal(data, &current); err == nil && schemaIsCurrent(current) {
		return current, nil
	}

	// Fall back to the legacy schema (shim → runtime name).
	var legacy map[string]string
	if err := json.Unmarshal(data, &legacy); err == nil {
		converted := make(ShimMap, len(legacy))
		for shim, runtime := range legacy {
			converted[shim] = ShimEntry{Runtime: runtime}
		}
		return converted, nil
	}

	return nil, fmt.Errorf("shim-map cache at %s is in an unrecognized format", cachePath)
}

// schemaIsCurrent reports whether a successfully-unmarshalled ShimMap was
// actually serialized with the current schema. A legacy file like
// `{"uv": "python"}` will technically unmarshal into ShimMap (every entry
// becomes a zero-valued ShimEntry), so a Runtime field that is empty for
// every entry indicates the input was actually legacy.
func schemaIsCurrent(m ShimMap) bool {
	if len(m) == 0 {
		return true
	}
	for _, entry := range m {
		if entry.Runtime != "" {
			return true
		}
	}
	return false
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
// For shims already in the cache, the runtime is overwritten with the new
// value (typically the same) and the providing-versions set is unioned —
// so installing a second runtime version that provides the same executable
// extends the version list rather than clobbering it. The in-memory cache
// is reset so subsequent LoadShimMap calls read the updated state from disk.
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

	for shim, entry := range entries {
		if cur, ok := existing[shim]; ok {
			cur.Runtime = entry.Runtime
			cur.Versions = unionVersions(cur.Versions, entry.Versions)
			existing[shim] = cur
		} else {
			existing[shim] = entry
		}
	}

	// Force the next LoadShimMap to re-read from disk so the merged entries
	// are visible to any subsequent caller in the same process.
	ResetShimMapCache()

	return SaveShimMap(existing)
}

// unionVersions returns a slice containing every distinct version from a
// and b, preserving the order in which versions are first seen.
func unionVersions(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, v := range a {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	for _, v := range b {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

// Lookup returns the full ShimEntry (runtime + providing versions) for a
// shim, or zero-value and false if the shim is not in the cache. Use this
// when you need the version coverage data; for callers that only need the
// runtime name, use LookupRuntime.
func Lookup(shimName string) (ShimEntry, bool) {
	shimMap, err := LoadShimMap()
	if err != nil {
		return ShimEntry{}, false
	}
	entry, ok := shimMap[shimName]
	return entry, ok
}

// LookupRuntime looks up the runtime for a given shim name using the cache.
// Returns the runtime name and true if found, or empty string and false if not.
//
// This is a convenience wrapper around Lookup for callers that don't need
// the providing-versions data.
func LookupRuntime(shimName string) (string, bool) {
	entry, ok := Lookup(shimName)
	return entry.Runtime, ok
}

// ResetShimMapCache resets the cached shim map, forcing a reload on next access.
// This is primarily useful for testing.
func ResetShimMapCache() {
	shimMapCacheOnce = sync.Once{}
	shimMapCache = nil
	shimMapCacheErr = nil
}
