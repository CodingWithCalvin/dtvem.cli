package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// DefaultCacheTTL is the default time-to-live for cached manifests.
const DefaultCacheTTL = 24 * time.Hour

// CachedSource wraps a Source and caches manifests locally.
// This is primarily useful for remote sources to avoid repeated network requests.
type CachedSource struct {
	source   Source
	cacheDir string
	ttl      time.Duration
	// version is the dtvem version reading and writing the cache. A cache
	// file written by another version is treated as missing.
	version string
}

// cliVersion is the running dtvem version, recorded by SetCLIVersion.
var cliVersion string

// SetCLIVersion records the running dtvem version. The default source's
// cache then ignores manifests cached by any other version, so installing a
// new dtvem always fetches fresh manifests instead of reading one cached
// before the new version's manifest fields existed. Call it before
// DefaultSource.
func SetCLIVersion(version string) {
	cliVersion = version
}

// cacheFormat is the format of the cache files this version writes. A cache
// file of another format is treated as missing: it holds the manifest as
// the CLI that wrote it parsed it, without the fields that CLI did not know.
//
//   - 0: no format field; written before builds and alternates existed
//   - 2: keeps Download.Build and Download.Alternates
const cacheFormat = 2

// cacheEntry stores a manifest along with its cache timestamp.
type cacheEntry struct {
	Format     int       `json:"format,omitempty"`
	CLIVersion string    `json:"cli_version,omitempty"`
	CachedAt   time.Time `json:"cached_at"`
	Manifest   *Manifest `json:"manifest"`
}

// NewCachedSource creates a Source that caches results from the underlying source.
func NewCachedSource(source Source, cacheDir string, ttl time.Duration) *CachedSource {
	return &CachedSource{
		source:   source,
		cacheDir: cacheDir,
		ttl:      ttl,
	}
}

// GetManifest returns a cached manifest if valid, otherwise fetches from the underlying source.
func (s *CachedSource) GetManifest(runtime string) (*Manifest, error) {
	// Try to load from cache first
	if manifest, err := s.loadFromCache(runtime); err == nil {
		return manifest, nil
	}

	// Fetch from underlying source
	manifest, err := s.source.GetManifest(runtime)
	if err != nil {
		return nil, err
	}

	// Save to cache (ignore errors, caching is best-effort)
	_ = s.saveToCache(runtime, manifest)

	return manifest, nil
}

// ListRuntimes delegates to the underlying source (not cached).
func (s *CachedSource) ListRuntimes() ([]string, error) {
	return s.source.ListRuntimes()
}

// ForceRefresh clears the cache and fetches fresh manifests.
func (s *CachedSource) ForceRefresh(runtime string) (*Manifest, error) {
	// Remove cached file
	cachePath := s.cachePath(runtime)
	_ = os.Remove(cachePath)

	// Fetch fresh from source
	manifest, err := s.source.GetManifest(runtime)
	if err != nil {
		return nil, err
	}

	// Save to cache
	_ = s.saveToCache(runtime, manifest)

	return manifest, nil
}

// ClearCache removes all cached manifests.
func (s *CachedSource) ClearCache() error {
	entries, err := os.ReadDir(s.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(s.cacheDir, entry.Name())
		if err := os.Remove(path); err != nil {
			return err
		}
	}

	return nil
}

func (s *CachedSource) cachePath(runtime string) string {
	return filepath.Join(s.cacheDir, runtime+".cache.json")
}

func (s *CachedSource) loadFromCache(runtime string) (*Manifest, error) {
	cachePath := s.cachePath(runtime)

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	// A cache written in another format, by another dtvem version, or too long
	// ago is treated as not found
	if entry.Format != cacheFormat || entry.CLIVersion != s.version || time.Since(entry.CachedAt) > s.ttl {
		return nil, os.ErrNotExist
	}

	return entry.Manifest, nil
}

func (s *CachedSource) saveToCache(runtime string, manifest *Manifest) error {
	if err := os.MkdirAll(s.cacheDir, 0755); err != nil {
		return err
	}

	entry := cacheEntry{
		Format:     cacheFormat,
		CLIVersion: s.version,
		CachedAt:   time.Now(),
		Manifest:   manifest,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(s.cachePath(runtime), data, 0644)
}
