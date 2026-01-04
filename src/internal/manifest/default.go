package manifest

import (
	"path/filepath"
	"sync"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
)

var (
	defaultSource     Source
	defaultCached     *CachedSource
	defaultEmbedded   *EmbeddedSource
	defaultSourceOnce sync.Once
)

// DefaultSource returns the default manifest source.
// It uses a cached remote source with embedded fallback:
//  1. Check local cache (24hr TTL)
//  2. Fetch from remote (manifests.dtvem.io)
//  3. Fall back to embedded manifests if remote fails
//
// The source is created once and reused for all subsequent calls.
func DefaultSource() Source {
	defaultSourceOnce.Do(func() {
		defaultSource = createDefaultSource()
	})
	return defaultSource
}

// createDefaultSource builds the layered source stack.
func createDefaultSource() Source {
	// Cache directory for manifest files
	paths := config.DefaultPaths()
	cacheDir := filepath.Join(paths.Cache, "manifests")

	// Remote source - fetches from manifests.dtvem.io
	remote := NewHTTPSource(DefaultRemoteURL)

	// Cached source - wraps remote with local disk cache
	defaultCached = NewCachedSource(remote, cacheDir, DefaultCacheTTL)

	// Embedded source - bundled in binary, always available
	defaultEmbedded = NewEmbeddedSource()

	// Fallback source - tries cached/remote first, falls back to embedded
	return NewFallbackSource(defaultCached, defaultEmbedded)
}

// ForceRefreshRuntime clears the cache for a specific runtime and fetches fresh data.
// Returns the refreshed manifest and whether it came from remote (true) or embedded (false).
func ForceRefreshRuntime(runtime string) (*Manifest, bool, error) {
	// Ensure default source is initialized
	DefaultSource()

	// Clear cache for this runtime
	if defaultCached != nil {
		m, err := defaultCached.ForceRefresh(runtime)
		if err == nil {
			return m, true, nil // Got from remote
		}
	}

	// Fall back to embedded
	if defaultEmbedded != nil {
		m, err := defaultEmbedded.GetManifest(runtime)
		if err == nil {
			return m, false, nil // Got from embedded
		}
		return nil, false, err
	}

	return nil, false, &ErrManifestNotFound{Runtime: runtime}
}

// ClearAllCache removes all cached manifests.
func ClearAllCache() error {
	// Ensure default source is initialized
	DefaultSource()

	if defaultCached != nil {
		return defaultCached.ClearCache()
	}
	return nil
}

// ListAvailableRuntimes returns all runtimes that have manifests available.
func ListAvailableRuntimes() ([]string, error) {
	// Use embedded source to list runtimes (most reliable)
	if defaultEmbedded == nil {
		DefaultSource() // Initialize if needed
	}
	if defaultEmbedded != nil {
		return defaultEmbedded.ListRuntimes()
	}
	return nil, nil
}

// ResetDefaultSource clears the cached default source.
// This is primarily useful for testing.
func ResetDefaultSource() {
	defaultSourceOnce = sync.Once{}
	defaultSource = nil
	defaultCached = nil
	defaultEmbedded = nil
}
