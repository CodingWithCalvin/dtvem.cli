package manifest

import (
	"github.com/dtvem/dtvem/src/internal/ui"
)

// FallbackSource tries multiple sources in order, falling back on failure.
// This enables graceful degradation when remote sources are unavailable.
type FallbackSource struct {
	primary  Source
	fallback Source
}

// NewFallbackSource creates a Source that tries the primary source first,
// then falls back to the fallback source if the primary fails.
func NewFallbackSource(primary, fallback Source) *FallbackSource {
	return &FallbackSource{
		primary:  primary,
		fallback: fallback,
	}
}

// GetManifest tries to get the manifest from the primary source,
// falling back to the fallback source on any error.
func (s *FallbackSource) GetManifest(runtime string) (*Manifest, error) {
	manifest, err := s.primary.GetManifest(runtime)
	if err == nil {
		return manifest, nil
	}

	// Log the fallback for debugging
	ui.Debug("Primary manifest source failed for %s: %v, falling back to embedded", runtime, err)

	// Try fallback
	return s.fallback.GetManifest(runtime)
}

// ListRuntimes tries to list runtimes from the primary source,
// falling back to the fallback source on any error.
func (s *FallbackSource) ListRuntimes() ([]string, error) {
	runtimes, err := s.primary.ListRuntimes()
	if err == nil {
		return runtimes, nil
	}

	// Try fallback
	return s.fallback.ListRuntimes()
}
