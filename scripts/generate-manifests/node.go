package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	nodeIndexURL   = "https://nodejs.org/dist/index.json"
	nodeDistURL    = "https://nodejs.org/dist"
	nodeSchemaURL  = "https://raw.githubusercontent.com/dtvem/dtvem/main/schemas/manifest.schema.json"
)

// nodeRelease represents a Node.js release from index.json
type nodeRelease struct {
	Version  string   `json:"version"`
	Date     string   `json:"date"`
	Files    []string `json:"files"`
	LTS      any      `json:"lts"` // false or string
	Security bool     `json:"security"`
}

// Platform mappings from Node.js file identifiers to our manifest platform keys
// Node.js uses identifiers like "win-x64-zip", "linux-x64", "osx-arm64-tar"
var nodePlatformMap = map[string]struct {
	platform string
	archive  string
}{
	"win-x64-zip":   {"windows-amd64", "zip"},
	"win-arm64-zip": {"windows-arm64", "zip"},
	"win-x86-zip":   {"windows-386", "zip"},
	"osx-x64-tar":   {"darwin-amd64", "tar.gz"},
	"osx-arm64-tar": {"darwin-arm64", "tar.gz"},
	"linux-x64":     {"linux-amd64", "tar.gz"},
	"linux-arm64":   {"linux-arm64", "tar.gz"},
	"linux-armv7l":  {"linux-arm", "tar.gz"},
}

func generateNodeManifest(outputDir string) error {
	fmt.Println("Generating Node.js manifest...")

	// Fetch version index
	releases, err := fetchNodeReleases()
	if err != nil {
		return fmt.Errorf("failed to fetch releases: %w", err)
	}

	fmt.Printf("Found %d releases\n", len(releases))

	manifest := &Manifest{
		Schema:   nodeSchemaURL,
		Version:  1,
		Versions: make(map[string]map[string]*Download),
	}

	// Process each release
	for i, release := range releases {
		version := strings.TrimPrefix(release.Version, "v")

		// Progress indicator
		if (i+1)%50 == 0 || i == len(releases)-1 {
			fmt.Printf("Processing %d/%d versions...\n", i+1, len(releases))
		}

		// Fetch checksums for this version
		checksums, err := fetchNodeChecksums(release.Version)
		if err != nil {
			fmt.Printf("Warning: failed to fetch checksums for %s: %v\n", version, err)
			continue
		}

		// Build platform map for this version
		platforms := make(map[string]*Download)

		for nodeFile, mapping := range nodePlatformMap {
			// Check if this platform is available for this version
			if !containsFile(release.Files, nodeFile) {
				continue
			}

			// Construct filename based on platform
			var filename string
			if strings.HasPrefix(nodeFile, "win-") {
				// Windows: node-v22.0.0-win-x64.zip
				// nodeFile is "win-x64-zip", we need "win-x64"
				winPlatform := strings.TrimSuffix(nodeFile, "-zip")
				filename = fmt.Sprintf("node-%s-%s.%s", release.Version, winPlatform, mapping.archive)
			} else if strings.HasPrefix(nodeFile, "osx-") {
				// macOS: node-v22.0.0-darwin-arm64.tar.gz
				// nodeFile is "osx-arm64-tar", we need "arm64"
				archPart := strings.TrimSuffix(strings.TrimPrefix(nodeFile, "osx-"), "-tar")
				filename = fmt.Sprintf("node-%s-darwin-%s.%s", release.Version, archPart, mapping.archive)
			} else {
				// Linux: node-v22.0.0-linux-x64.tar.gz
				filename = fmt.Sprintf("node-%s-%s.%s", release.Version, nodeFile, mapping.archive)
			}

			// Look up checksum
			sha256, ok := checksums[filename]
			if !ok {
				continue
			}

			platforms[mapping.platform] = &Download{
				URL:    fmt.Sprintf("%s/%s/%s", nodeDistURL, release.Version, filename),
				SHA256: sha256,
			}
		}

		if len(platforms) > 0 {
			manifest.Versions[version] = platforms
		}
	}

	fmt.Printf("Generated manifest with %d versions\n", len(manifest.Versions))

	return writeManifest(manifest, outputDir, "node.json")
}

func fetchNodeReleases() ([]nodeRelease, error) {
	resp, err := http.Get(nodeIndexURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var releases []nodeRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	return releases, nil
}

func fetchNodeChecksums(version string) (map[string]string, error) {
	url := fmt.Sprintf("%s/%s/SHASUMS256.txt", nodeDistURL, version)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	checksums := make(map[string]string)
	scanner := bufio.NewScanner(resp.Body)

	for scanner.Scan() {
		line := scanner.Text()
		// Format: <sha256>  <filename>
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			sha256 := parts[0]
			filename := parts[len(parts)-1] // Handle paths like win-x64/node.exe
			// Only include top-level files (not subdirectory files)
			if !strings.Contains(filename, "/") {
				checksums[filename] = sha256
			}
		}
	}

	return checksums, scanner.Err()
}

func containsFile(files []string, target string) bool {
	for _, f := range files {
		if f == target {
			return true
		}
	}
	return false
}
