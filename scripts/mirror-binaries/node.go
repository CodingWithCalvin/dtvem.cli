package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const nodeIndexURL = "https://nodejs.org/dist/index.json"

// NodeOfficialSource fetches Node.js versions from nodejs.org
type NodeOfficialSource struct{}

func (s *NodeOfficialSource) Name() string {
	return "nodejs.org"
}

// nodeIndexEntry represents an entry in nodejs.org/dist/index.json
type nodeIndexEntry struct {
	Version string   `json:"version"`
	Date    string   `json:"date"`
	Files   []string `json:"files"`
	LTS     any      `json:"lts"` // Can be string or false
	Shasums string   `json:"shasums,omitempty"`
}

// nodeShasums maps filename to SHA256 checksum
type nodeShasums map[string]string

func (s *NodeOfficialSource) FetchVersions() ([]MirrorJob, error) {
	// Fetch index.json
	resp, err := httpClient.Get(nodeIndexURL)
	if err != nil {
		return nil, fmt.Errorf("fetching index: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching index: HTTP %d", resp.StatusCode)
	}

	var entries []nodeIndexEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		return nil, fmt.Errorf("parsing index: %w", err)
	}

	var jobs []MirrorJob

	for _, entry := range entries {
		version := strings.TrimPrefix(entry.Version, "v")

		// Fetch checksums for this version
		shasums, err := s.fetchShasums(entry.Version)
		if err != nil {
			// Older versions may not have SHASUMS256, continue without checksums
			shasums = nil
		}

		// Map Node.js file types to our platform naming
		for _, file := range entry.Files {
			platform, ext := s.mapFileToPlatform(file)
			if platform == "" {
				continue // Skip unsupported file types
			}

			archiveName := s.getArchiveName(entry.Version, file)
			url := fmt.Sprintf("https://nodejs.org/dist/%s/%s", entry.Version, archiveName)

			var sha256 string
			if shasums != nil {
				sha256 = shasums[archiveName]
			}

			r2Key := fmt.Sprintf("node/%s/%s%s", version, platform, ext)
			metaKey := fmt.Sprintf("node/%s/%s.meta.json", version, platform)

			jobs = append(jobs, MirrorJob{
				Runtime:        "node",
				Version:        version,
				Platform:       platform,
				URL:            url,
				UpstreamSHA256: sha256,
				R2Key:          r2Key,
				MetaKey:        metaKey,
			})
		}
	}

	return jobs, nil
}

func (s *NodeOfficialSource) fetchShasums(version string) (nodeShasums, error) {
	url := fmt.Sprintf("https://nodejs.org/dist/%s/SHASUMS256.txt", version)
	resp, err := httpClient.Get(url)
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

	shasums := make(nodeShasums)
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: "checksum  filename" (two spaces)
		parts := strings.SplitN(line, "  ", 2)
		if len(parts) == 2 {
			shasums[parts[1]] = parts[0]
		}
	}

	return shasums, nil
}

func (s *NodeOfficialSource) mapFileToPlatform(file string) (platform, ext string) {
	// Node.js file naming: linux-x64, darwin-x64, win-x64, etc.
	// We want: linux-amd64, darwin-amd64, windows-amd64, etc.

	switch file {
	// Linux
	case "linux-x64":
		return "linux-amd64", ".tar.gz"
	case "linux-arm64":
		return "linux-arm64", ".tar.gz"
	case "linux-armv7l":
		return "linux-armv7", ".tar.gz"

	// macOS
	case "darwin-x64":
		return "darwin-amd64", ".tar.gz"
	case "darwin-arm64":
		return "darwin-arm64", ".tar.gz"

	// Windows
	case "win-x64-zip":
		return "windows-amd64", ".zip"
	case "win-arm64-zip":
		return "windows-arm64", ".zip"
	case "win-x86-zip":
		return "windows-386", ".zip"

	default:
		// Skip MSI installers, source tarballs, headers, etc.
		return "", ""
	}
}

func (s *NodeOfficialSource) getArchiveName(version, file string) string {
	// Convert file type to actual archive filename
	switch file {
	case "linux-x64":
		return fmt.Sprintf("node-%s-linux-x64.tar.gz", version)
	case "linux-arm64":
		return fmt.Sprintf("node-%s-linux-arm64.tar.gz", version)
	case "linux-armv7l":
		return fmt.Sprintf("node-%s-linux-armv7l.tar.gz", version)
	case "darwin-x64":
		return fmt.Sprintf("node-%s-darwin-x64.tar.gz", version)
	case "darwin-arm64":
		return fmt.Sprintf("node-%s-darwin-arm64.tar.gz", version)
	case "win-x64-zip":
		return fmt.Sprintf("node-%s-win-x64.zip", version)
	case "win-arm64-zip":
		return fmt.Sprintf("node-%s-win-arm64.zip", version)
	case "win-x86-zip":
		return fmt.Sprintf("node-%s-win-x86.zip", version)
	default:
		return ""
	}
}
