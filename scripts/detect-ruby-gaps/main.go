package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Platform describes a target platform with runner metadata for GitHub Actions
type Platform struct {
	Name    string
	Runner  string
	BuildOS string
	Arch    string
}

// MatrixEntry represents a single entry in the GitHub Actions matrix
type MatrixEntry struct {
	Version  string `json:"version"`
	Platform string `json:"platform"`
	Runner   string `json:"runner"`
	BuildOS  string `json:"build_os"`
	Arch     string `json:"arch"`
}

// MatrixOutput is the JSON structure consumed by GitHub Actions fromJson()
type MatrixOutput struct {
	Include []MatrixEntry `json:"include"`
}

var allPlatforms = []Platform{
	{Name: "linux-amd64", Runner: "ubuntu-latest", BuildOS: "linux", Arch: "amd64"},
	{Name: "linux-arm64", Runner: "ubuntu-24.04-arm", BuildOS: "linux", Arch: "arm64"},
	{Name: "darwin-amd64", Runner: "macos-13", BuildOS: "darwin", Arch: "amd64"},
	{Name: "darwin-arm64", Runner: "macos-latest", BuildOS: "darwin", Arch: "arm64"},
	{Name: "windows-amd64", Runner: "windows-latest", BuildOS: "windows", Arch: "amd64"},
	{Name: "windows-386", Runner: "windows-latest", BuildOS: "windows", Arch: "386"},
}

var (
	versionFlag = flag.String("version", "", "Force all 6 platforms for this version (no R2 check)")
	r2Endpoint  = flag.String("r2-endpoint", "", "R2 endpoint URL")
	r2Bucket    = flag.String("r2-bucket", "", "R2 bucket name")
	r2AccessKey = flag.String("r2-access-key", "", "R2 access key ID")
	r2SecretKey = flag.String("r2-secret-key", "", "R2 secret access key")
)

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

// httpClient is a shared HTTP client with reasonable timeouts
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

func main() {
	flag.Parse()

	// If --version is provided, output all 6 platforms without R2 check
	if *versionFlag != "" {
		matrix := buildMatrixForVersion(*versionFlag)
		outputMatrix(matrix)
		return
	}

	// Otherwise, detect gaps by comparing upstream vs R2
	if *r2Endpoint == "" || *r2Bucket == "" || *r2AccessKey == "" || *r2SecretKey == "" {
		fmt.Fprintln(os.Stderr, "Error: R2 credentials required (--r2-endpoint, --r2-bucket, --r2-access-key, --r2-secret-key)")
		fmt.Fprintln(os.Stderr, "  Or use --version=X to force all platforms for a specific version")
		os.Exit(1)
	}

	// Fetch known versions from upstream sources
	fmt.Fprintln(os.Stderr, "Fetching Ruby versions from upstream sources...")
	knownVersions, err := fetchKnownVersions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching upstream versions: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Found %d unique Ruby versions from upstream\n", len(knownVersions))

	// Fetch existing metadata from R2
	fmt.Fprintln(os.Stderr, "Fetching existing metadata from R2...")
	existingMeta, err := fetchExistingMeta()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error fetching R2 metadata: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "Found %d existing metadata entries in R2\n", len(existingMeta))

	// Compute gaps
	matrix := computeGaps(knownVersions, existingMeta)
	fmt.Fprintf(os.Stderr, "Detected %d gaps\n", len(matrix.Include))

	outputMatrix(matrix)
}

// buildMatrixForVersion creates a matrix with all 6 platforms for a given version
func buildMatrixForVersion(version string) *MatrixOutput {
	matrix := &MatrixOutput{}
	for _, p := range allPlatforms {
		// Exclude darwin-arm64 for versions < 3.1.0
		if p.Name == "darwin-arm64" && !supportsARM64Darwin(version) {
			continue
		}
		matrix.Include = append(matrix.Include, MatrixEntry{
			Version:  version,
			Platform: p.Name,
			Runner:   p.Runner,
			BuildOS:  p.BuildOS,
			Arch:     p.Arch,
		})
	}
	return matrix
}

// supportsARM64Darwin returns true if the version supports ARM64 macOS (>= 3.1.0)
func supportsARM64Darwin(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major := 0
	minor := 0
	fmt.Sscanf(parts[0], "%d", &major)
	fmt.Sscanf(parts[1], "%d", &minor)

	if major > 3 {
		return true
	}
	if major == 3 && minor >= 1 {
		return true
	}
	return false
}

// fetchKnownVersions queries upstream sources and returns a sorted list of unique versions
func fetchKnownVersions() ([]string, error) {
	versionSet := make(map[string]bool)

	// Fetch from RubyInstaller (Windows builds)
	fmt.Fprintln(os.Stderr, "  Fetching from RubyInstaller...")
	installerVersions, err := fetchRubyInstallerVersions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: failed to fetch from RubyInstaller: %v\n", err)
	} else {
		for _, v := range installerVersions {
			versionSet[v] = true
		}
		fmt.Fprintf(os.Stderr, "  Found %d versions from RubyInstaller\n", len(installerVersions))
	}

	// Fetch from ruby-builder (Linux/macOS builds)
	fmt.Fprintln(os.Stderr, "  Fetching from ruby-builder...")
	builderVersions, err := fetchRubyBuilderVersions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  Warning: failed to fetch from ruby-builder: %v\n", err)
	} else {
		for _, v := range builderVersions {
			versionSet[v] = true
		}
		fmt.Fprintf(os.Stderr, "  Found %d versions from ruby-builder\n", len(builderVersions))
	}

	// Filter: only >= 2.7.0, exclude preview/rc
	var versions []string
	for v := range versionSet {
		if isPreRelease(v) {
			continue
		}
		if !isAtLeast270(v) {
			continue
		}
		versions = append(versions, v)
	}

	sort.Strings(versions)
	return versions, nil
}

// rubyInstallerPattern matches filenames like: rubyinstaller-3.2.2-1-x64.7z
var rubyInstallerPattern = regexp.MustCompile(
	`^rubyinstaller-(\d+\.\d+\.\d+)-\d+-([^.]+)\.(7z|zip)$`,
)

func fetchRubyInstallerVersions() ([]string, error) {
	url := "https://api.github.com/repos/oneclick/rubyinstaller2/releases?per_page=100"
	resp, err := httpGetWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("parsing releases: %w", err)
	}

	seen := make(map[string]bool)
	var versions []string
	for _, release := range releases {
		for _, asset := range release.Assets {
			matches := rubyInstallerPattern.FindStringSubmatch(asset.Name)
			if matches == nil {
				continue
			}
			version := matches[1]
			if !seen[version] {
				seen[version] = true
				versions = append(versions, version)
			}
		}
	}

	return versions, nil
}

// rubyBuilderPattern matches filenames like: ruby-3.2.2-ubuntu-22.04.tar.gz
var rubyBuilderPattern = regexp.MustCompile(
	`^ruby-(\d+\.\d+\.\d+)-([^.]+(?:\.[^.]+)?(?:-arm64)?)\.(tar\.gz)$`,
)

func fetchRubyBuilderVersions() ([]string, error) {
	url := "https://api.github.com/repos/ruby/ruby-builder/releases/tags/toolcache"
	resp, err := httpGetWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("fetching release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("parsing release: %w", err)
	}

	seen := make(map[string]bool)
	var versions []string
	for _, asset := range release.Assets {
		matches := rubyBuilderPattern.FindStringSubmatch(asset.Name)
		if matches == nil {
			continue
		}
		version := matches[1]
		if !seen[version] {
			seen[version] = true
			versions = append(versions, version)
		}
	}

	return versions, nil
}

// isPreRelease returns true if the version contains a pre-release suffix (e.g., "-preview1", "-rc1")
func isPreRelease(version string) bool {
	return strings.Contains(version, "-")
}

// isAtLeast270 returns true if the version is >= 2.7.0
func isAtLeast270(version string) bool {
	parts := strings.SplitN(version, ".", 3)
	if len(parts) < 2 {
		return false
	}
	major := 0
	minor := 0
	fmt.Sscanf(parts[0], "%d", &major)
	fmt.Sscanf(parts[1], "%d", &minor)

	if major > 2 {
		return true
	}
	if major == 2 && minor >= 7 {
		return true
	}
	return false
}

// fetchExistingMeta lists all ruby/**/*.meta.json keys in R2
func fetchExistingMeta() (map[string]bool, error) {
	client, err := createS3Client()
	if err != nil {
		return nil, fmt.Errorf("creating S3 client: %w", err)
	}

	keys := make(map[string]bool)
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: r2Bucket,
		Prefix: aws.String("ruby/"),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("listing objects: %w", err)
		}
		for _, obj := range page.Contents {
			key := *obj.Key
			if strings.HasSuffix(key, ".meta.json") {
				keys[key] = true
			}
		}
	}

	return keys, nil
}

// metaKeyPattern matches paths like "ruby/3.2.10/linux-amd64.meta.json"
var metaKeyPattern = regexp.MustCompile(`^ruby/([^/]+)/([^/]+)\.meta\.json$`)

// computeGaps determines which version+platform pairs are missing from R2
func computeGaps(versions []string, existingMeta map[string]bool) *MatrixOutput {
	matrix := &MatrixOutput{}

	for _, version := range versions {
		for _, p := range allPlatforms {
			// Exclude darwin-arm64 for versions < 3.1.0
			if p.Name == "darwin-arm64" && !supportsARM64Darwin(version) {
				continue
			}

			metaKey := fmt.Sprintf("ruby/%s/%s.meta.json", version, p.Name)
			if !existingMeta[metaKey] {
				matrix.Include = append(matrix.Include, MatrixEntry{
					Version:  version,
					Platform: p.Name,
					Runner:   p.Runner,
					BuildOS:  p.BuildOS,
					Arch:     p.Arch,
				})
			}
		}
	}

	return matrix
}

func outputMatrix(matrix *MatrixOutput) {
	data, err := json.Marshal(matrix)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling matrix: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

func createS3Client() (*s3.Client, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			*r2AccessKey,
			*r2SecretKey,
			"",
		)),
		config.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(*r2Endpoint)
	})

	return client, nil
}

func httpGetWithRetry(url string, maxRetries int) (*http.Response, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := httpClient.Get(url)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
			continue
		}

		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
			continue
		}

		return resp, nil
	}
	return nil, lastErr
}
