package main

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// RubyInstallerSource fetches Ruby versions from rubyinstaller2 (Windows)
type RubyInstallerSource struct{}

func (s *RubyInstallerSource) Name() string {
	return "rubyinstaller2"
}

// rubyInstallerPattern matches filenames like:
// rubyinstaller-3.2.2-1-x64.7z
var rubyInstallerPattern = regexp.MustCompile(
	`^rubyinstaller-(\d+\.\d+\.\d+)-\d+-([^.]+)\.(7z|zip)$`,
)

func (s *RubyInstallerSource) FetchVersions() ([]MirrorJob, error) {
	// Fetch releases from GitHub API with retries
	url := "https://api.github.com/repos/oneclick/rubyinstaller2/releases?per_page=100"
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
		for _, asset := range release.Assets {
			matches := rubyInstallerPattern.FindStringSubmatch(asset.Name)
			if matches == nil {
				continue
			}

			version := matches[1]
			arch := matches[2]
			ext := "." + matches[3]

			platform := s.mapArchToPlatform(arch)
			if platform == "" {
				continue
			}

			// Skip duplicates
			key := version + "/" + platform
			if seen[key] {
				continue
			}
			seen[key] = true

			r2Key := fmt.Sprintf("ruby/%s/%s%s", version, platform, ext)
			metaKey := fmt.Sprintf("ruby/%s/%s.meta.json", version, platform)

			jobs = append(jobs, MirrorJob{
				Runtime:        "ruby",
				Version:        version,
				Platform:       platform,
				URL:            asset.BrowserDownloadURL,
				UpstreamSHA256: "", // RubyInstaller doesn't provide checksums in releases
				R2Key:          r2Key,
				MetaKey:        metaKey,
			})
		}
	}

	return jobs, nil
}

func (s *RubyInstallerSource) mapArchToPlatform(arch string) string {
	switch arch {
	case "x64":
		return "windows-amd64"
	case "x86":
		return "windows-386"
	default:
		return ""
	}
}

// RubyBuilderSource fetches Ruby versions from ruby/ruby-builder (Linux/macOS)
type RubyBuilderSource struct{}

func (s *RubyBuilderSource) Name() string {
	return "ruby-builder"
}

// rubyBuilderPattern matches filenames like:
// ruby-3.2.2-ubuntu-22.04.tar.gz
// ruby-3.2.2-macos-latest.tar.gz
// ruby-3.2.2-macos-13-arm64.tar.gz
var rubyBuilderPattern = regexp.MustCompile(
	`^ruby-(\d+\.\d+\.\d+)-([^.]+(?:\.[^.]+)?(?:-arm64)?)\.(tar\.gz)$`,
)

func (s *RubyBuilderSource) FetchVersions() ([]MirrorJob, error) {
	// Fetch the toolcache release from GitHub API with retries
	url := "https://api.github.com/repos/ruby/ruby-builder/releases/tags/toolcache"
	resp, err := httpGetWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("fetching release: HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release: %w", err)
	}

	var jobs []MirrorJob
	seen := make(map[string]bool)

	for _, asset := range release.Assets {
		matches := rubyBuilderPattern.FindStringSubmatch(asset.Name)
		if matches == nil {
			continue
		}

		version := matches[1]
		osArch := matches[2]
		ext := "." + matches[3]

		platform := s.mapOsArchToPlatform(osArch)
		if platform == "" {
			continue
		}

		// Skip duplicates (prefer specific versions like ubuntu-22.04 over ubuntu-latest)
		key := version + "/" + platform
		if seen[key] {
			continue
		}
		seen[key] = true

		r2Key := fmt.Sprintf("ruby/%s/%s%s", version, platform, ext)
		metaKey := fmt.Sprintf("ruby/%s/%s.meta.json", version, platform)

		jobs = append(jobs, MirrorJob{
			Runtime:        "ruby",
			Version:        version,
			Platform:       platform,
			URL:            asset.BrowserDownloadURL,
			UpstreamSHA256: "", // ruby-builder doesn't provide checksums
			R2Key:          r2Key,
			MetaKey:        metaKey,
		})
	}

	return jobs, nil
}

func (s *RubyBuilderSource) mapOsArchToPlatform(osArch string) string {
	switch {
	// Linux (prefer ubuntu-22.04 as it's most compatible)
	case strings.HasPrefix(osArch, "ubuntu"):
		if strings.Contains(osArch, "arm64") {
			return "linux-arm64"
		}
		return "linux-amd64"

	// macOS
	case strings.HasPrefix(osArch, "macos"):
		if strings.Contains(osArch, "arm64") {
			return "darwin-arm64"
		}
		return "darwin-amd64"

	default:
		return ""
	}
}
