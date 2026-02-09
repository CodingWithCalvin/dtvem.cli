package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// BinaryMeta represents metadata stored alongside each binary
type BinaryMeta struct {
	SHA256       string `json:"sha256"`
	SHA256Source string `json:"sha256_source"` // "upstream" or "dtvem"
	SourceURL    string `json:"source_url"`
	MirroredAt   string `json:"mirrored_at"`
	Size         int64  `json:"size"`
}

// ManifestDownload represents a download entry in the manifest
type ManifestDownload struct {
	URL          string `json:"url"`
	SHA256       string `json:"sha256,omitempty"`
	SHA256Source string `json:"sha256_source,omitempty"`
	Source       string `json:"source,omitempty"` // "built-from-source" when not from upstream
}

// Manifest represents the output manifest structure
type Manifest struct {
	Version  int                                      `json:"version"`
	Versions map[string]map[string]*ManifestDownload `json:"versions"`
}

var (
	runtimeFlag  = flag.String("runtime", "", "Runtime to generate (node, python, ruby, or all)")
	outputDir    = flag.String("output-dir", "src/internal/manifest/data", "Output directory for manifests")
	baseURL      = flag.String("base-url", "https://builds.dtvem.io", "Base URL for binary downloads")
	r2Endpoint   = flag.String("r2-endpoint", "", "R2 endpoint URL")
	r2Bucket     = flag.String("r2-bucket", "", "R2 bucket name")
	r2AccessKey  = flag.String("r2-access-key", "", "R2 access key ID")
	r2SecretKey  = flag.String("r2-secret-key", "", "R2 secret access key")
	dryRun       = flag.Bool("dry-run", false, "Report what would be generated without writing files")
)

// metaKeyPattern matches paths like "node/20.18.0/linux-amd64.meta.json"
var metaKeyPattern = regexp.MustCompile(`^([^/]+)/([^/]+)/([^/]+)\.meta\.json$`)

func main() {
	flag.Parse()

	if *runtimeFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: --runtime is required (node, python, ruby, or all)")
		os.Exit(1)
	}

	if *r2Endpoint == "" || *r2Bucket == "" || *r2AccessKey == "" || *r2SecretKey == "" {
		fmt.Fprintln(os.Stderr, "Error: R2 credentials required (--r2-endpoint, --r2-bucket, --r2-access-key, --r2-secret-key)")
		os.Exit(1)
	}

	runtimes := []string{*runtimeFlag}
	if *runtimeFlag == "all" {
		runtimes = []string{"node", "python", "ruby"}
	}

	// Create S3 client
	s3Client, err := createS3Client()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating S3 client: %v\n", err)
		os.Exit(1)
	}

	// Generate manifest for each runtime
	for _, runtime := range runtimes {
		fmt.Printf("Generating manifest for %s...\n", runtime)

		manifest, err := generateManifest(s3Client, runtime)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating manifest for %s: %v\n", runtime, err)
			os.Exit(1)
		}

		if *dryRun {
			fmt.Printf("\n[DRY RUN] Would generate %s.json with %d versions\n", runtime, len(manifest.Versions))
			// Print sample of versions
			count := 0
			for version, platforms := range manifest.Versions {
				if count >= 5 {
					fmt.Printf("  ... and %d more versions\n", len(manifest.Versions)-5)
					break
				}
				fmt.Printf("  %s: %d platforms\n", version, len(platforms))
				count++
			}
			continue
		}

		// Write manifest file
		outputPath := filepath.Join(*outputDir, runtime+".json")
		if err := writeManifest(manifest, outputPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing manifest for %s: %v\n", runtime, err)
			os.Exit(1)
		}

		fmt.Printf("  Written %s with %d versions\n", outputPath, len(manifest.Versions))
	}

	fmt.Println("\nManifest generation complete!")
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

func generateManifest(client *s3.Client, runtime string) (*Manifest, error) {
	manifest := &Manifest{
		Version:  1,
		Versions: make(map[string]map[string]*ManifestDownload),
	}

	// List all files for this runtime, separating meta files from binary keys
	prefix := runtime + "/"
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: r2Bucket,
		Prefix: aws.String(prefix),
	})

	metaFiles := []string{}
	allKeys := make(map[string]bool)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, fmt.Errorf("listing objects: %w", err)
		}

		for _, obj := range page.Contents {
			key := *obj.Key
			allKeys[key] = true
			if strings.HasSuffix(key, ".meta.json") {
				metaFiles = append(metaFiles, key)
			}
		}
	}

	fmt.Printf("  Found %d metadata files\n", len(metaFiles))

	// Process each metadata file
	for _, metaKey := range metaFiles {
		// Parse the key to extract runtime, version, platform
		matches := metaKeyPattern.FindStringSubmatch(metaKey)
		if matches == nil {
			fmt.Printf("  Warning: skipping invalid meta key: %s\n", metaKey)
			continue
		}

		rt := matches[1]
		version := matches[2]
		platform := matches[3]

		if rt != runtime {
			continue
		}

		// Download and parse metadata
		meta, err := downloadMeta(client, metaKey)
		if err != nil {
			fmt.Printf("  Warning: failed to read metadata for %s: %v\n", metaKey, err)
			continue
		}

		// Determine the binary file extension from source URL
		ext := getExtension(meta.SourceURL)
		if !isValidArchiveExt(ext) {
			// Source URL doesn't contain a valid extension (e.g., "built-from-source"),
			// probe R2 for the actual binary file
			ext = findBinaryExtension(allKeys, runtime, version, platform)
			if ext == "" {
				fmt.Printf("  Warning: no valid binary found for %s/%s/%s, skipping\n", runtime, version, platform)
				continue
			}
		}
		binaryURL := fmt.Sprintf("%s/%s/%s/%s%s", *baseURL, runtime, version, platform, ext)

		// Add to manifest
		if manifest.Versions[version] == nil {
			manifest.Versions[version] = make(map[string]*ManifestDownload)
		}

		download := &ManifestDownload{
			URL:          binaryURL,
			SHA256:       meta.SHA256,
			SHA256Source: meta.SHA256Source,
		}
		if meta.SourceURL == "built-from-source" {
			download.Source = "built-from-source"
		}

		manifest.Versions[version][platform] = download
	}

	return manifest, nil
}

func downloadMeta(client *s3.Client, key string) (*BinaryMeta, error) {
	resp, err := client.GetObject(context.Background(), &s3.GetObjectInput{
		Bucket: r2Bucket,
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var meta BinaryMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}

func getExtension(url string) string {
	// Handle common archive extensions
	if strings.HasSuffix(url, ".tar.gz") {
		return ".tar.gz"
	}
	if strings.HasSuffix(url, ".tar.xz") {
		return ".tar.xz"
	}
	if strings.HasSuffix(url, ".tar.bz2") {
		return ".tar.bz2"
	}
	if strings.HasSuffix(url, ".zip") {
		return ".zip"
	}
	if strings.HasSuffix(url, ".7z") {
		return ".7z"
	}
	// Fallback: extract from URL
	base := filepath.Base(url)
	if idx := strings.Index(base, "."); idx != -1 {
		return base[idx:]
	}
	return ""
}

func isValidArchiveExt(ext string) bool {
	switch ext {
	case ".tar.gz", ".tar.xz", ".tar.bz2", ".zip", ".7z":
		return true
	}
	return false
}

func findBinaryExtension(allKeys map[string]bool, runtime, version, platform string) string {
	prefix := fmt.Sprintf("%s/%s/%s", runtime, version, platform)
	for _, ext := range []string{".tar.gz", ".tar.xz", ".zip", ".7z"} {
		if allKeys[prefix+ext] {
			return ext
		}
	}
	return ""
}

func writeManifest(manifest *Manifest, path string) error {
	// Sort versions for consistent output
	sortedManifest := &Manifest{
		Version:  manifest.Version,
		Versions: make(map[string]map[string]*ManifestDownload),
	}

	// Get sorted version keys
	versions := make([]string, 0, len(manifest.Versions))
	for v := range manifest.Versions {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	for _, v := range versions {
		platforms := manifest.Versions[v]
		sortedManifest.Versions[v] = make(map[string]*ManifestDownload)

		// Get sorted platform keys
		platformKeys := make([]string, 0, len(platforms))
		for p := range platforms {
			platformKeys = append(platformKeys, p)
		}
		sort.Strings(platformKeys)

		for _, p := range platformKeys {
			sortedManifest.Versions[v][p] = platforms[p]
		}
	}

	data, err := json.MarshalIndent(sortedManifest, "", "  ")
	if err != nil {
		return err
	}

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
