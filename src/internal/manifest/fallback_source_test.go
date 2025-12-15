package manifest

import (
	"errors"
	"testing"
)

// fallbackTestSource is a test helper that returns predefined responses.
type fallbackTestSource struct {
	manifest *Manifest
	runtimes []string
	err      error
}

func (s *fallbackTestSource) GetManifest(_ string) (*Manifest, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.manifest, nil
}

func (s *fallbackTestSource) ListRuntimes() ([]string, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.runtimes, nil
}

func TestFallbackSource(t *testing.T) {
	primaryManifest := &Manifest{
		Version: 1,
		Versions: map[string]map[string]*Download{
			"3.13.1": {
				"windows-amd64": {URL: "https://primary.com/python.zip", SHA256: "primary"},
			},
		},
	}

	fallbackManifest := &Manifest{
		Version: 1,
		Versions: map[string]map[string]*Download{
			"3.12.0": {
				"windows-amd64": {URL: "https://fallback.com/python.zip", SHA256: "fallback"},
			},
		},
	}

	t.Run("uses primary when successful", func(t *testing.T) {
		primary := &fallbackTestSource{manifest: primaryManifest}
		fallback := &fallbackTestSource{manifest: fallbackManifest}
		source := NewFallbackSource(primary, fallback)

		m, err := source.GetManifest("python")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should use primary
		d := m.GetDownload("3.13.1", "windows-amd64")
		if d == nil {
			t.Fatal("expected download from primary")
		}
		if d.SHA256 != "primary" {
			t.Errorf("SHA256 = %q, want %q", d.SHA256, "primary")
		}
	})

	t.Run("falls back on primary error", func(t *testing.T) {
		primary := &fallbackTestSource{err: errors.New("network error")}
		fallback := &fallbackTestSource{manifest: fallbackManifest}
		source := NewFallbackSource(primary, fallback)

		m, err := source.GetManifest("python")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Should use fallback
		d := m.GetDownload("3.12.0", "windows-amd64")
		if d == nil {
			t.Fatal("expected download from fallback")
		}
		if d.SHA256 != "fallback" {
			t.Errorf("SHA256 = %q, want %q", d.SHA256, "fallback")
		}
	})

	t.Run("returns error when both fail", func(t *testing.T) {
		primary := &fallbackTestSource{err: errors.New("primary error")}
		fallback := &fallbackTestSource{err: errors.New("fallback error")}
		source := NewFallbackSource(primary, fallback)

		_, err := source.GetManifest("python")
		if err == nil {
			t.Fatal("expected error when both sources fail")
		}
	})

	t.Run("ListRuntimes uses primary when successful", func(t *testing.T) {
		primary := &fallbackTestSource{runtimes: []string{"python", "node"}}
		fallback := &fallbackTestSource{runtimes: []string{"ruby"}}
		source := NewFallbackSource(primary, fallback)

		runtimes, err := source.ListRuntimes()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runtimes) != 2 {
			t.Errorf("len(runtimes) = %d, want 2", len(runtimes))
		}
	})

	t.Run("ListRuntimes falls back on primary error", func(t *testing.T) {
		primary := &fallbackTestSource{err: errors.New("primary error")}
		fallback := &fallbackTestSource{runtimes: []string{"ruby"}}
		source := NewFallbackSource(primary, fallback)

		runtimes, err := source.ListRuntimes()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(runtimes) != 1 || runtimes[0] != "ruby" {
			t.Errorf("runtimes = %v, want [ruby]", runtimes)
		}
	})
}

func TestFallbackSourceWithNotFoundError(t *testing.T) {
	// When primary returns ErrManifestNotFound, should still fall back
	primary := &fallbackTestSource{err: &ErrManifestNotFound{Runtime: "python"}}
	fallback := &fallbackTestSource{manifest: &Manifest{Version: 1, Versions: map[string]map[string]*Download{}}}
	source := NewFallbackSource(primary, fallback)

	m, err := source.GetManifest("python")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected manifest from fallback")
	}
}
