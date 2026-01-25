package main

import (
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// PythonStandaloneSource fetches Python versions from astral-sh/python-build-standalone
type PythonStandaloneSource struct{}

func (s *PythonStandaloneSource) Name() string {
	return "python-build-standalone"
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
}

// pythonStandalonePattern matches filenames like:
// cpython-3.12.0+20231002-x86_64-unknown-linux-gnu-install_only.tar.gz
var pythonStandalonePattern = regexp.MustCompile(
	`^cpython-(\d+\.\d+\.\d+)\+\d+-([^-]+-[^-]+-[^-]+(?:-[^-]+)?)-install_only\.(tar\.gz|tar\.zst)$`,
)

func (s *PythonStandaloneSource) FetchVersions() ([]MirrorJob, error) {
	// Fetch releases from GitHub API with retries
	url := "https://api.github.com/repos/astral-sh/python-build-standalone/releases?per_page=100"
	resp, err := httpGetWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching releases: HTTP %d", resp.StatusCode)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}

	var jobs []MirrorJob
	seen := make(map[string]bool)

	for _, release := range releases {
		// Also fetch SHA256 sums if available
		shasums := s.fetchShasums(release)

		for _, asset := range release.Assets {
			matches := pythonStandalonePattern.FindStringSubmatch(asset.Name)
			if matches == nil {
				continue
			}

			version := matches[1]
			triple := matches[2]
			ext := "." + matches[3]

			platform := s.mapTripleToPlatform(triple)
			if platform == "" {
				continue
			}

			// Skip duplicates (prefer first occurrence which is newest release)
			key := version + "/" + platform
			if seen[key] {
				continue
			}
			seen[key] = true

			r2Key := fmt.Sprintf("python/%s/%s%s", version, platform, ext)
			metaKey := fmt.Sprintf("python/%s/%s.meta.json", version, platform)

			jobs = append(jobs, MirrorJob{
				Runtime:        "python",
				Version:        version,
				Platform:       platform,
				URL:            asset.BrowserDownloadURL,
				UpstreamSHA256: shasums[asset.Name],
				R2Key:          r2Key,
				MetaKey:        metaKey,
			})
		}
	}

	return jobs, nil
}

func (s *PythonStandaloneSource) fetchShasums(release githubRelease) map[string]string {
	shasums := make(map[string]string)

	// Look for SHA256SUMS file in release assets
	for _, asset := range release.Assets {
		if asset.Name == "SHA256SUMS" {
			resp, err := httpClient.Get(asset.BrowserDownloadURL)
			if err != nil {
				return shasums
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return shasums
			}

			for _, line := range strings.Split(string(body), "\n") {
				parts := strings.Fields(line)
				if len(parts) >= 2 {
					shasums[parts[1]] = parts[0]
				}
			}
			break
		}
	}

	return shasums
}

func (s *PythonStandaloneSource) mapTripleToPlatform(triple string) string {
	// Map rust-style triples to our platform naming
	switch {
	// Linux
	case strings.Contains(triple, "x86_64") && strings.Contains(triple, "linux"):
		return "linux-amd64"
	case strings.Contains(triple, "aarch64") && strings.Contains(triple, "linux"):
		return "linux-arm64"

	// macOS
	case strings.Contains(triple, "x86_64") && strings.Contains(triple, "apple"):
		return "darwin-amd64"
	case strings.Contains(triple, "aarch64") && strings.Contains(triple, "apple"):
		return "darwin-arm64"

	// Windows
	case strings.Contains(triple, "x86_64") && strings.Contains(triple, "windows"):
		return "windows-amd64"
	case strings.Contains(triple, "i686") && strings.Contains(triple, "windows"):
		return "windows-386"

	default:
		return ""
	}
}

// PythonOfficialSource fetches Python versions from python.org
// This is a fallback for versions not available in python-build-standalone
type PythonOfficialSource struct{}

func (s *PythonOfficialSource) Name() string {
	return "python.org"
}

func (s *PythonOfficialSource) FetchVersions() ([]MirrorJob, error) {
	// Python.org doesn't provide prebuilt binaries for most platforms
	// Only Windows installers and source tarballs are available
	// For now, we rely primarily on python-build-standalone
	// This source can be expanded later if needed

	// The official FTP has a complex structure:
	// https://www.python.org/ftp/python/3.12.0/
	// - Python-3.12.0.tar.xz (source)
	// - python-3.12.0-amd64.exe (Windows installer - not a portable archive)
	// - python-3.12.0-embed-amd64.zip (Windows embeddable - limited use)

	// For now, return empty - python-build-standalone covers our needs
	return []MirrorJob{}, nil
}
