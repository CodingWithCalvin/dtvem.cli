package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	pythonReleasesURL = "https://api.github.com/repos/astral-sh/python-build-standalone/releases"
	pythonSchemaURL   = "https://raw.githubusercontent.com/dtvem/dtvem/main/schemas/manifest.schema.json"
)

// Platform mappings from python-build-standalone naming to our manifest platform keys
// We use the "install_only" variant for simplicity
var pythonPlatformMap = map[string]string{
	"x86_64-pc-windows-msvc":    "windows-amd64",
	"aarch64-pc-windows-msvc":   "windows-arm64",
	"x86_64-apple-darwin":       "darwin-amd64",
	"aarch64-apple-darwin":      "darwin-arm64",
	"x86_64-unknown-linux-gnu":  "linux-amd64",
	"aarch64-unknown-linux-gnu": "linux-arm64",
}

// Regex to parse asset names like: cpython-3.13.1+20251209-x86_64-unknown-linux-gnu-install_only.tar.gz
var pythonAssetRegex = regexp.MustCompile(`^cpython-(\d+\.\d+\.\d+)\+\d+-(.+)-install_only\.tar\.gz$`)

func generatePythonManifest(outputDir string) error {
	fmt.Println("Generating Python manifest...")

	// Fetch releases from GitHub API
	releases, err := fetchPythonReleases()
	if err != nil {
		return fmt.Errorf("failed to fetch releases: %w", err)
	}

	fmt.Printf("Found %d releases\n", len(releases))

	manifest := &Manifest{
		Schema:   pythonSchemaURL,
		Version:  1,
		Versions: make(map[string]map[string]*Download),
	}

	// Process each release
	for _, release := range releases {
		fmt.Printf("Processing release %s (%d assets)...\n", release.TagName, len(release.Assets))

		for _, asset := range release.Assets {
			// Parse asset name to extract version and platform
			matches := pythonAssetRegex.FindStringSubmatch(asset.Name)
			if matches == nil {
				continue
			}

			version := matches[1]     // e.g., "3.13.1"
			pbsPlatform := matches[2] // e.g., "x86_64-unknown-linux-gnu"

			// Map to our platform key
			platform, ok := pythonPlatformMap[pbsPlatform]
			if !ok {
				continue
			}

			// Extract SHA256 from digest if available (format: "sha256:abc123...")
			sha256 := ""
			if strings.HasPrefix(asset.Digest, "sha256:") {
				sha256 = strings.TrimPrefix(asset.Digest, "sha256:")
			}

			// Initialize version map if needed
			if manifest.Versions[version] == nil {
				manifest.Versions[version] = make(map[string]*Download)
			}

			// Only add if we don't already have this version/platform
			// (prefer newer releases which come first from the API)
			if manifest.Versions[version][platform] == nil {
				manifest.Versions[version][platform] = &Download{
					URL:    asset.BrowserDownloadURL,
					SHA256: sha256,
				}
			}
		}
	}

	fmt.Printf("Generated manifest with %d versions\n", len(manifest.Versions))

	return writeManifest(manifest, outputDir, "python.json")
}

func fetchPythonReleases() ([]githubRelease, error) {
	// Use smaller page size for python-build-standalone due to large number of assets per release
	return fetchGitHubReleasesWithPageSize(pythonReleasesURL, 10)
}
