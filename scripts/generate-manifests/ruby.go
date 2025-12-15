package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// ruby-builder provides macOS and Linux builds
	rubyBuilderReleasesURL = "https://api.github.com/repos/ruby/ruby-builder/releases"
	// RubyInstaller provides Windows builds
	rubyInstallerReleasesURL = "https://api.github.com/repos/oneclick/rubyinstaller2/releases"
	rubySchemaURL            = "https://raw.githubusercontent.com/dtvem/dtvem/main/schemas/manifest.schema.json"
)

// Platform mappings from ruby-builder naming to our manifest platform keys
// Prefer ubuntu-22.04 for broader compatibility
var rubyBuilderPlatformMap = map[string]string{
	"darwin-arm64":       "darwin-arm64",
	"darwin-x64":         "darwin-amd64",
	"ubuntu-22.04-x64":   "linux-amd64",
	"ubuntu-22.04-arm64": "linux-arm64",
	// Fallback to ubuntu-24.04 if 22.04 not available
	"ubuntu-24.04-x64":   "linux-amd64",
	"ubuntu-24.04-arm64": "linux-arm64",
}

// Platform mappings from RubyInstaller naming to our manifest platform keys
var rubyInstallerPlatformMap = map[string]string{
	"x64": "windows-amd64",
	// x86 builds are not included (32-bit Windows)
}

// Regex to parse ruby-builder asset names like: ruby-3.3.10-ubuntu-22.04-x64.tar.gz
// Captures: version, suffix type, platform (e.g., "ubuntu-22.04-x64" or "darwin-arm64")
// The version suffix only matches known pre-release patterns (preview, rc, alpha, beta, dev)
// to avoid capturing platform names like "darwin"
var rubyBuilderAssetRegex = regexp.MustCompile(`^ruby-(\d+\.\d+\.\d+(?:-(preview|rc|alpha|beta|dev)\d*)?)-(.+)\.tar\.gz$`)

// Regex to parse RubyInstaller asset names like: rubyinstaller-3.3.10-1-x64.7z
// Captures: version, build number (unused), architecture
var rubyInstallerAssetRegex = regexp.MustCompile(`^rubyinstaller-(\d+\.\d+\.\d+)-\d+-(\w+)\.7z$`)

func generateRubyManifest(outputDir string) error {
	fmt.Println("Generating Ruby manifest...")

	manifest := &Manifest{
		Schema:   rubySchemaURL,
		Version:  1,
		Versions: make(map[string]map[string]*Download),
	}

	// Track which version/platform combos we've already added
	// to prefer ubuntu-22.04 over ubuntu-24.04
	added := make(map[string]bool)

	// Fetch and process ruby-builder releases (macOS + Linux)
	builderReleases, err := fetchRubyBuilderReleases()
	if err != nil {
		return fmt.Errorf("failed to fetch ruby-builder releases: %w", err)
	}
	fmt.Printf("Found %d ruby-builder releases\n", len(builderReleases))
	processRubyBuilderReleases(builderReleases, manifest, added)

	// Fetch and process RubyInstaller releases (Windows)
	installerReleases, err := fetchRubyInstallerReleases()
	if err != nil {
		return fmt.Errorf("failed to fetch RubyInstaller releases: %w", err)
	}
	fmt.Printf("Found %d RubyInstaller releases\n", len(installerReleases))
	processRubyInstallerReleases(installerReleases, manifest, added)

	fmt.Printf("Generated manifest with %d versions\n", len(manifest.Versions))

	return writeManifest(manifest, outputDir, "ruby.json")
}

// processRubyBuilderReleases processes ruby-builder releases for macOS and Linux
func processRubyBuilderReleases(releases []githubRelease, manifest *Manifest, added map[string]bool) {
	for _, release := range releases {
		for _, asset := range release.Assets {
			// Skip non-standard Ruby builds (truffleruby, jruby, etc.)
			if !strings.HasPrefix(asset.Name, "ruby-") {
				continue
			}

			// Parse asset name to extract version and platform
			matches := rubyBuilderAssetRegex.FindStringSubmatch(asset.Name)
			if matches == nil {
				continue
			}

			version := matches[1]    // e.g., "3.3.10" or "4.0.0-preview2"
			// matches[2] is the suffix type (preview, rc, etc.) - not used
			rbPlatform := matches[3] // e.g., "ubuntu-22.04-x64"

			// Map to our platform key
			platform, ok := rubyBuilderPlatformMap[rbPlatform]
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

			// Create a unique key for tracking
			key := version + "/" + platform

			// Only add if we don't already have this version/platform
			// This ensures we prefer ubuntu-22.04 over ubuntu-24.04 (processed first)
			if !added[key] {
				manifest.Versions[version][platform] = &Download{
					URL:    asset.BrowserDownloadURL,
					SHA256: sha256,
				}
				added[key] = true
			}
		}
	}
}

// processRubyInstallerReleases processes RubyInstaller releases for Windows
// Note: Older RubyInstaller releases don't have SHA256 digests (artifact attestations),
// but we still include them for broader Windows support
func processRubyInstallerReleases(releases []githubRelease, manifest *Manifest, added map[string]bool) {
	for _, release := range releases {
		for _, asset := range release.Assets {
			// Parse asset name to extract version and architecture
			matches := rubyInstallerAssetRegex.FindStringSubmatch(asset.Name)
			if matches == nil {
				continue
			}

			version := matches[1] // e.g., "3.3.10"
			arch := matches[2]    // e.g., "x64"

			// Map to our platform key
			platform, ok := rubyInstallerPlatformMap[arch]
			if !ok {
				continue
			}

			// Extract SHA256 from digest if available (format: "sha256:abc123...")
			// Note: Only recent releases have artifact attestations with digests
			sha256 := ""
			if strings.HasPrefix(asset.Digest, "sha256:") {
				sha256 = strings.TrimPrefix(asset.Digest, "sha256:")
			}

			// Initialize version map if needed
			if manifest.Versions[version] == nil {
				manifest.Versions[version] = make(map[string]*Download)
			}

			// Create a unique key for tracking
			key := version + "/" + platform

			// Only add if we don't already have this version/platform
			if !added[key] {
				manifest.Versions[version][platform] = &Download{
					URL:    asset.BrowserDownloadURL,
					SHA256: sha256,
				}
				added[key] = true
			}
		}
	}
}

func fetchRubyBuilderReleases() ([]githubRelease, error) {
	return fetchAllGitHubReleases(rubyBuilderReleasesURL)
}

func fetchRubyInstallerReleases() ([]githubRelease, error) {
	return fetchAllGitHubReleases(rubyInstallerReleasesURL)
}
