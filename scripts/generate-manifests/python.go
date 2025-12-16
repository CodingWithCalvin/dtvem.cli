package main

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const (
	pythonReleasesURL = "https://api.github.com/repos/astral-sh/python-build-standalone/releases"
	pythonOrgFTPURL   = "https://www.python.org/ftp/python/"
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

// Regex to parse python.org version directories
var pythonOrgVersionRegex = regexp.MustCompile(`href="(\d+\.\d+\.\d+)/"`)

// Regex to parse python.org embeddable package names
var pythonOrgEmbedRegex = regexp.MustCompile(`href="(python-(\d+\.\d+\.\d+)-embed-(amd64|arm64)\.zip)"`)

func generatePythonManifest(outputDir string) error {
	fmt.Println("Generating Python manifest...")

	manifest := &Manifest{
		Schema:   pythonSchemaURL,
		Version:  1,
		Versions: make(map[string]map[string]*Download),
	}

	// First, fetch from python-build-standalone (primary source for Linux/macOS, newer Windows)
	if err := fetchPythonBuildStandalone(manifest); err != nil {
		fmt.Printf("Warning: failed to fetch python-build-standalone: %v\n", err)
	}

	// Then, fetch from python.org for Windows (fills gaps for older versions)
	if err := fetchPythonOrg(manifest); err != nil {
		fmt.Printf("Warning: failed to fetch python.org: %v\n", err)
	}

	fmt.Printf("Generated manifest with %d versions\n", len(manifest.Versions))

	return writeManifest(manifest, outputDir, "python.json")
}

// fetchPythonBuildStandalone fetches releases from astral-sh/python-build-standalone
func fetchPythonBuildStandalone(manifest *Manifest) error {
	fmt.Println("Fetching from python-build-standalone...")

	releases, err := fetchPythonReleases()
	if err != nil {
		return fmt.Errorf("failed to fetch releases: %w", err)
	}

	fmt.Printf("Found %d releases from python-build-standalone\n", len(releases))

	for _, release := range releases {
		for _, asset := range release.Assets {
			matches := pythonAssetRegex.FindStringSubmatch(asset.Name)
			if matches == nil {
				continue
			}

			version := matches[1]
			pbsPlatform := matches[2]

			platform, ok := pythonPlatformMap[pbsPlatform]
			if !ok {
				continue
			}

			sha256 := ""
			if strings.HasPrefix(asset.Digest, "sha256:") {
				sha256 = strings.TrimPrefix(asset.Digest, "sha256:")
			}

			if manifest.Versions[version] == nil {
				manifest.Versions[version] = make(map[string]*Download)
			}

			if manifest.Versions[version][platform] == nil {
				manifest.Versions[version][platform] = &Download{
					URL:    asset.BrowserDownloadURL,
					SHA256: sha256,
				}
			}
		}
	}

	return nil
}

// fetchPythonOrg fetches Windows embeddable packages from python.org
func fetchPythonOrg(manifest *Manifest) error {
	fmt.Println("Fetching Windows packages from python.org...")

	// Get list of versions from python.org FTP
	versions, err := listPythonOrgVersions()
	if err != nil {
		return fmt.Errorf("failed to list versions: %w", err)
	}

	fmt.Printf("Found %d versions on python.org\n", len(versions))

	addedCount := 0
	for _, version := range versions {
		// Skip if we already have Windows builds for this version
		if manifest.Versions[version] != nil && manifest.Versions[version]["windows-amd64"] != nil {
			continue
		}

		// Check for embeddable packages
		packages, err := listPythonOrgPackages(version)
		if err != nil {
			continue // Skip versions without packages
		}

		for _, pkg := range packages {
			if manifest.Versions[version] == nil {
				manifest.Versions[version] = make(map[string]*Download)
			}

			// Only add if we don't already have this platform
			if manifest.Versions[version][pkg.Platform] == nil {
				manifest.Versions[version][pkg.Platform] = &Download{
					URL:    pkg.URL,
					SHA256: "", // python.org doesn't provide easy SHA256 access
				}
				addedCount++
			}
		}
	}

	fmt.Printf("Added %d Windows packages from python.org\n", addedCount)
	return nil
}

// listPythonOrgVersions lists available Python versions from python.org FTP
func listPythonOrgVersions() ([]string, error) {
	resp, err := http.Get(pythonOrgFTPURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	matches := pythonOrgVersionRegex.FindAllStringSubmatch(string(body), -1)
	versions := make([]string, 0, len(matches))
	for _, m := range matches {
		version := m[1]
		// Only include Python 3.x versions (skip 2.x)
		if strings.HasPrefix(version, "3.") {
			versions = append(versions, version)
		}
	}

	return versions, nil
}

// pythonOrgPackage represents a package from python.org
type pythonOrgPackage struct {
	URL      string
	Platform string
}

// listPythonOrgPackages lists embeddable packages for a specific version
func listPythonOrgPackages(version string) ([]pythonOrgPackage, error) {
	url := pythonOrgFTPURL + version + "/"
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	matches := pythonOrgEmbedRegex.FindAllStringSubmatch(string(body), -1)
	packages := make([]pythonOrgPackage, 0, len(matches))

	for _, m := range matches {
		filename := m[1]
		arch := m[3]

		platform := ""
		switch arch {
		case "amd64":
			platform = "windows-amd64"
		case "arm64":
			platform = "windows-arm64"
		default:
			continue
		}

		packages = append(packages, pythonOrgPackage{
			URL:      url + filename,
			Platform: platform,
		})
	}

	return packages, nil
}

func fetchPythonReleases() ([]githubRelease, error) {
	// Use smaller page size for python-build-standalone due to large number of assets per release
	return fetchGitHubReleasesWithPageSize(pythonReleasesURL, 10)
}
