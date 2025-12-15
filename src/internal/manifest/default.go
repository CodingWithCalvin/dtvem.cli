package manifest

import (
	"path/filepath"
	"sync"

	"github.com/dtvem/dtvem/src/internal/config"
)

var (
	defaultSource     Source
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
	cached := NewCachedSource(remote, cacheDir, DefaultCacheTTL)

	// Embedded source - bundled in binary, always available
	embedded := NewEmbeddedSource()

	// Fallback source - tries cached/remote first, falls back to embedded
	return NewFallbackSource(cached, embedded)
}

// ResetDefaultSource clears the cached default source.
// This is primarily useful for testing.
func ResetDefaultSource() {
	defaultSourceOnce = sync.Once{}
	defaultSource = nil
}
