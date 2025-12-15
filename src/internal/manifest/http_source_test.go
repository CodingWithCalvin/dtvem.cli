package manifest

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPSource(t *testing.T) {
	pythonManifest := `{
		"version": 1,
		"versions": {
			"3.13.1": {
				"windows-amd64": {"url": "https://example.com/python.zip", "sha256": "abc123"}
			}
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/python.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(pythonManifest))
		case "/ruby.json":
			w.WriteHeader(http.StatusNotFound)
		case "/broken.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("invalid json"))
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	source := NewHTTPSource(server.URL)

	t.Run("GetManifest success", func(t *testing.T) {
		m, err := source.GetManifest("python")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if m == nil {
			t.Fatal("expected manifest, got nil")
		}
		if len(m.Versions) != 1 {
			t.Errorf("len(Versions) = %d, want 1", len(m.Versions))
		}
		d := m.GetDownload("3.13.1", "windows-amd64")
		if d == nil {
			t.Fatal("expected download info")
		}
		if d.URL != "https://example.com/python.zip" {
			t.Errorf("URL = %q, want %q", d.URL, "https://example.com/python.zip")
		}
	})

	t.Run("GetManifest not found", func(t *testing.T) {
		_, err := source.GetManifest("ruby")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !IsManifestNotFound(err) {
			t.Errorf("expected ErrManifestNotFound, got %T: %v", err, err)
		}
	})

	t.Run("GetManifest invalid JSON", func(t *testing.T) {
		_, err := source.GetManifest("broken")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		// Should be a parse error, not a not found error
		if IsManifestNotFound(err) {
			t.Errorf("should not be ErrManifestNotFound, got %v", err)
		}
	})

	t.Run("GetManifest server error", func(t *testing.T) {
		_, err := source.GetManifest("unknown")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})

	t.Run("ListRuntimes not supported", func(t *testing.T) {
		_, err := source.ListRuntimes()
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

func TestHTTPSourceWithClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version": 1, "versions": {}}`))
	}))
	defer server.Close()

	customClient := &http.Client{}
	source := NewHTTPSourceWithClient(server.URL, customClient)

	m, err := source.GetManifest("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected manifest")
	}
}

func TestHTTPSourceNetworkError(t *testing.T) {
	// Use a URL that will fail to connect
	source := NewHTTPSource("http://localhost:1")

	_, err := source.GetManifest("python")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}
