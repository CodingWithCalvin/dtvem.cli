package manifest

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultRemoteURL is the default URL for fetching manifests.
const DefaultRemoteURL = "https://manifests.dtvem.io"

// DefaultHTTPTimeout is the default timeout for HTTP requests.
const DefaultHTTPTimeout = 30 * time.Second

// HTTPSource fetches manifests from a remote HTTP server.
type HTTPSource struct {
	baseURL    string
	httpClient *http.Client
}

// NewHTTPSource creates a Source that fetches manifests from a remote URL.
func NewHTTPSource(baseURL string) *HTTPSource {
	return &HTTPSource{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: DefaultHTTPTimeout,
		},
	}
}

// NewHTTPSourceWithClient creates an HTTPSource with a custom HTTP client.
// This is useful for testing or custom timeout/transport configuration.
func NewHTTPSourceWithClient(baseURL string, client *http.Client) *HTTPSource {
	return &HTTPSource{
		baseURL:    baseURL,
		httpClient: client,
	}
}

// GetManifest fetches and parses a manifest from the remote server.
func (s *HTTPSource) GetManifest(runtime string) (*Manifest, error) {
	url := fmt.Sprintf("%s/%s.json", s.baseURL, runtime)

	resp, err := s.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch manifest: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, &ErrManifestNotFound{Runtime: runtime}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch manifest: HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest response: %w", err)
	}

	return ParseManifest(data)
}

// ListRuntimes is not supported for HTTP sources.
// Remote manifests don't have a directory listing endpoint.
func (s *HTTPSource) ListRuntimes() ([]string, error) {
	// Could potentially fetch an index.json in the future,
	// but for now we don't support listing from remote.
	return nil, fmt.Errorf("ListRuntimes not supported for HTTP source")
}
