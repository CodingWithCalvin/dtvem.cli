package manifest

import (
	"testing"
)

func TestDefaultSource(t *testing.T) {
	// Reset any cached state
	ResetDefaultSource()
	defer ResetDefaultSource()

	t.Run("returns a source", func(t *testing.T) {
		source := DefaultSource()
		if source == nil {
			t.Fatal("expected source, got nil")
		}
	})

	t.Run("returns same instance on repeated calls", func(t *testing.T) {
		source1 := DefaultSource()
		source2 := DefaultSource()
		if source1 != source2 {
			t.Error("expected same instance")
		}
	})

	t.Run("can get manifest from embedded fallback", func(t *testing.T) {
		// Since the remote server doesn't exist yet, this will fall back to embedded
		source := DefaultSource()

		// This should work because embedded manifests are available
		m, err := source.GetManifest("node")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m == nil {
			t.Fatal("expected manifest")
		}
		if len(m.Versions) == 0 {
			t.Error("expected versions in manifest")
		}
	})
}

func TestResetDefaultSource(t *testing.T) {
	// Get initial source
	source1 := DefaultSource()

	// Reset
	ResetDefaultSource()

	// Get new source
	source2 := DefaultSource()

	// Should be different instances after reset
	if source1 == source2 {
		t.Error("expected different instance after reset")
	}
}
