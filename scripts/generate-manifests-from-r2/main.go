package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
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

	// Build names whose build of the runtime this is, for runtimes with
	// more than one (Python: "python-build-standalone" or "python.org").
	Build string `json:"build,omitempty"`

	// Alternates are other builds of the same version for the same
	// platform, which dtvem installs only when asked for by name. CLIs
	// released before alternates existed ignore them.
	Alternates []*ManifestDownload `json:"alternates,omitempty"`
}

// Build names, as recorded in ManifestDownload.Build.
const (
	buildPythonStandalone = "python-build-standalone"
	buildPythonOrg        = "python.org"
)

// alternateBuildDirs are the folders under <runtime>/<version>/ that hold
// builds other than the default, mapped to the build each holds.
var alternateBuildDirs = map[string]string{
	"python/python.org": buildPythonOrg,
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

// metaKeyPattern matches a version's default build, like
// "node/20.18.0/linux-amd64.meta.json", and a build in a folder of its own,
// like "python/3.12.10/python.org/windows-amd64.meta.json".
var metaKeyPattern = regexp.MustCompile(`^([^/]+)/([^/]+)/(?:([^/]+)/)?([^/]+)\.meta\.json$`)

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

	metas := make(map[string]*BinaryMeta)
	for _, metaKey := range metaFiles {
		meta, err := downloadMeta(client, metaKey)
		if err != nil {
			fmt.Printf("  Warning: failed to read metadata for %s: %v\n", metaKey, err)
			continue
		}
		metas[metaKey] = meta
	}

	return assembleManifest(runtime, *baseURL, metas, allKeys), nil
}

// manifestEntry is one build found in R2: a metadata file and where its
// binary is.
type manifestEntry struct {
	version  string
	platform string
	// keyBase is the binary's key without its extension, e.g.
	// "python/3.12.10/python.org/windows-amd64".
	keyBase string
	// build is the build this entry is, for builds in a folder of their own
	// ("" for a version's default build).
	build string
	meta  *BinaryMeta
}

// assembleManifest builds a runtime's manifest from the metadata files in
// R2 (keyed by their R2 key) and the set of every key under the runtime.
//
// Each version and platform gets its default build, labeled with whose
// build it is where that is known. Builds in a folder of their own are
// listed as alternates of the default; where there is no default build,
// one of them takes its place, and one that is the same build as the
// default is left out.
func assembleManifest(runtime, baseURL string, metas map[string]*BinaryMeta, allKeys map[string]bool) *Manifest {
	manifest := &Manifest{
		Version:  1,
		Versions: make(map[string]map[string]*ManifestDownload),
	}

	var defaults, alternates []manifestEntry
	for metaKey, meta := range metas {
		matches := metaKeyPattern.FindStringSubmatch(metaKey)
		if matches == nil {
			fmt.Printf("  Warning: skipping invalid meta key: %s\n", metaKey)
			continue
		}
		rt, version, dir, platform := matches[1], matches[2], matches[3], matches[4]
		if rt != runtime {
			continue
		}

		entry := manifestEntry{version: version, platform: platform, meta: meta}
		if dir == "" {
			entry.keyBase = fmt.Sprintf("%s/%s/%s", runtime, version, platform)
			defaults = append(defaults, entry)
			continue
		}
		build, ok := alternateBuildDirs[runtime+"/"+dir]
		if !ok {
			fmt.Printf("  Warning: skipping %s, in unknown build folder %q\n", metaKey, dir)
			continue
		}
		entry.keyBase = fmt.Sprintf("%s/%s/%s/%s", runtime, version, dir, platform)
		entry.build = build
		alternates = append(alternates, entry)
	}

	for _, e := range defaults {
		download := newManifestDownload(runtime, baseURL, e, allKeys)
		if download == nil {
			continue
		}
		if manifest.Versions[e.version] == nil {
			manifest.Versions[e.version] = make(map[string]*ManifestDownload)
		}
		manifest.Versions[e.version][e.platform] = download
	}

	// Attach alternates in a fixed order, so the output does not depend on
	// map iteration.
	sort.Slice(alternates, func(i, j int) bool { return alternates[i].keyBase < alternates[j].keyBase })
	for _, e := range alternates {
		download := newManifestDownload(runtime, baseURL, e, allKeys)
		if download == nil {
			continue
		}
		if manifest.Versions[e.version] == nil {
			manifest.Versions[e.version] = make(map[string]*ManifestDownload)
		}
		def := manifest.Versions[e.version][e.platform]
		switch {
		case def == nil:
			manifest.Versions[e.version][e.platform] = download
		case def.Build == download.Build:
			// The default already is this build.
		default:
			def.Alternates = append(def.Alternates, download)
		}
	}

	return manifest
}

// newManifestDownload returns the manifest entry for one build, or nil when
// its binary cannot be found in R2.
func newManifestDownload(runtime, baseURL string, e manifestEntry, allKeys map[string]bool) *ManifestDownload {
	// The source URL's extension is the binary's, unless dtvem built or
	// repacked it (built from source, or a Windows Python assembled from
	// python.org's files); then R2 has the binary under another extension.
	ext := getExtension(e.meta.SourceURL)
	if !isValidArchiveExt(ext) || !allKeys[e.keyBase+ext] {
		probed := findBinaryExtension(allKeys, e.keyBase)
		switch {
		case probed != "":
			ext = probed
		case !isValidArchiveExt(ext):
			fmt.Printf("  Warning: no valid binary found for %s, skipping\n", e.keyBase)
			return nil
		}
	}

	download := &ManifestDownload{
		URL:          fmt.Sprintf("%s/%s%s", baseURL, e.keyBase, ext),
		SHA256:       e.meta.SHA256,
		SHA256Source: e.meta.SHA256Source,
		Build:        e.build,
	}
	if e.meta.SourceURL == "built-from-source" {
		download.Source = "built-from-source"
	}
	if download.Build == "" {
		download.Build = buildFromSourceURL(runtime, e.meta.SourceURL)
	}
	return download
}

// buildFromSourceURL names whose build a binary is, from the upstream URL
// the mirror took it from. It returns "" for runtimes with a single build.
func buildFromSourceURL(runtime, sourceURL string) string {
	if runtime != "python" {
		return ""
	}
	u, err := url.Parse(sourceURL)
	if err != nil {
		return ""
	}
	switch {
	case strings.Contains(u.Path, "/python-build-standalone/"):
		return buildPythonStandalone
	case u.Host == "www.python.org", u.Host == "python.org":
		// The MSIs, install-manager zips and embeddable zips
		return buildPythonOrg
	case u.Host == "api.nuget.org" && strings.Contains(u.Path, "/python"):
		// python.org's NuGet packages
		return buildPythonOrg
	}
	return ""
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

func getExtension(sourceURL string) string {
	// Handle common archive extensions
	if strings.HasSuffix(sourceURL, ".tar.gz") {
		return ".tar.gz"
	}
	if strings.HasSuffix(sourceURL, ".tar.xz") {
		return ".tar.xz"
	}
	if strings.HasSuffix(sourceURL, ".tar.bz2") {
		return ".tar.bz2"
	}
	if strings.HasSuffix(sourceURL, ".zip") {
		return ".zip"
	}
	if strings.HasSuffix(sourceURL, ".7z") {
		return ".7z"
	}
	// Fallback: extract from URL
	base := filepath.Base(sourceURL)
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

// findBinaryExtension returns the extension of the binary at keyBase
// ("python/3.12.10/windows-amd64") in R2, or "" when there is none.
func findBinaryExtension(allKeys map[string]bool, keyBase string) string {
	for _, ext := range []string{".tar.gz", ".tar.xz", ".zip", ".7z"} {
		if allKeys[keyBase+ext] {
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
