// Script to generate manifest files from upstream sources.
// Run with: go run ./scripts/generate-manifests [node|python|ruby|all]
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run ./scripts/generate-manifests [node|python|ruby|all]")
		os.Exit(1)
	}

	runtime := os.Args[1]

	// Determine output directory (relative to repo root)
	outputDir := "src/internal/manifest/data"
	if len(os.Args) > 2 {
		outputDir = os.Args[2]
	}

	switch runtime {
	case "node":
		if err := generateNodeManifest(outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating Node.js manifest: %v\n", err)
			os.Exit(1)
		}
	case "python":
		if err := generatePythonManifest(outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating Python manifest: %v\n", err)
			os.Exit(1)
		}
	case "ruby":
		if err := generateRubyManifest(outputDir); err != nil {
			fmt.Fprintf(os.Stderr, "Error generating Ruby manifest: %v\n", err)
			os.Exit(1)
		}
	case "all":
		var errors []string
		if err := generateNodeManifest(outputDir); err != nil {
			errors = append(errors, fmt.Sprintf("Node.js: %v", err))
		}
		if err := generatePythonManifest(outputDir); err != nil {
			errors = append(errors, fmt.Sprintf("Python: %v", err))
		}
		if err := generateRubyManifest(outputDir); err != nil {
			errors = append(errors, fmt.Sprintf("Ruby: %v", err))
		}
		if len(errors) > 0 {
			fmt.Fprintf(os.Stderr, "Errors:\n%s\n", strings.Join(errors, "\n"))
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown runtime: %s\n", runtime)
		os.Exit(1)
	}

	fmt.Println("Done!")
}

// Manifest represents our manifest JSON structure
type Manifest struct {
	Schema   string                            `json:"$schema,omitempty"`
	Version  int                               `json:"version"`
	Versions map[string]map[string]*Download   `json:"versions"`
}

// Download contains URL and SHA256 for a binary
type Download struct {
	URL    string `json:"url"`
	SHA256 string `json:"sha256"`
}

// githubRelease represents a GitHub release
type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

// githubAsset represents a GitHub release asset
type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	// SHA256 digest is in the format "sha256:abc123..."
	Digest string `json:"digest"`
}

// writeManifest writes a manifest to a JSON file
func writeManifest(m *Manifest, outputDir, filename string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	path := filepath.Join(outputDir, filename)
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	// Add trailing newline
	data = append(data, '\n')

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	fmt.Printf("Wrote %s\n", path)
	return nil
}

// fetchAllGitHubReleases fetches all releases from a GitHub API URL, paginating through all pages
func fetchAllGitHubReleases(baseURL string) ([]githubRelease, error) {
	return fetchGitHubReleasesWithPageSize(baseURL, 100)
}

// fetchGitHubReleasesWithPageSize fetches all releases with a custom page size
// Smaller page sizes may be needed for repos with large responses (many assets per release)
func fetchGitHubReleasesWithPageSize(baseURL string, pageSize int) ([]githubRelease, error) {
	var allReleases []githubRelease
	url := fmt.Sprintf("%s?per_page=%d", baseURL, pageSize)

	for url != "" {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

		// GitHub API recommends setting Accept header
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("User-Agent", "dtvem-manifest-generator")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		var releases []githubRelease
		if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()

		allReleases = append(allReleases, releases...)

		// Check for next page in Link header
		url = getNextPageURL(resp.Header.Get("Link"))
	}

	return allReleases, nil
}

// getNextPageURL parses the Link header to find the next page URL
// Link header format: <url>; rel="next", <url>; rel="last"
func getNextPageURL(linkHeader string) string {
	if linkHeader == "" {
		return ""
	}

	links := strings.Split(linkHeader, ",")
	for _, link := range links {
		parts := strings.Split(strings.TrimSpace(link), ";")
		if len(parts) != 2 {
			continue
		}

		url := strings.Trim(strings.TrimSpace(parts[0]), "<>")
		rel := strings.TrimSpace(parts[1])

		if rel == `rel="next"` {
			return url
		}
	}

	return ""
}
