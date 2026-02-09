package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

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

// MirrorJob represents a single file to mirror
type MirrorJob struct {
	Runtime        string
	Version        string
	Platform       string
	URL            string
	UpstreamSHA256 string // Checksum from upstream manifest (may be empty)
	R2Key          string
	MetaKey        string
}

// Stats tracks mirroring statistics
type Stats struct {
	Total             int64
	Skipped           int64
	Mirrored          int64
	Failed            int64
	BytesDown         int64
	UpstreamChecksum  int64
	GeneratedChecksum int64
}

var (
	runtimeFlag = flag.String("runtime", "", "Runtime to mirror (node, python, ruby, or all)")
	dryRun      = flag.Bool("dry-run", false, "Report what would be done without doing it")
	syncOnly    = flag.Bool("sync-only", false, "Only mirror files not already in R2")
	r2Endpoint  = flag.String("r2-endpoint", "", "R2 endpoint URL")
	r2Bucket    = flag.String("r2-bucket", "", "R2 bucket name")
	r2AccessKey = flag.String("r2-access-key", "", "R2 access key ID")
	r2SecretKey = flag.String("r2-secret-key", "", "R2 secret access key")
	workers     = flag.Int("workers", 10, "Number of parallel workers")
	retries     = flag.Int("retries", 3, "Number of retries for failed downloads")
)

func main() {
	flag.Parse()

	if *runtimeFlag == "" {
		fmt.Fprintln(os.Stderr, "Error: --runtime is required (node, python, ruby, or all)")
		os.Exit(1)
	}

	if !*dryRun {
		if *r2Endpoint == "" || *r2Bucket == "" || *r2AccessKey == "" || *r2SecretKey == "" {
			fmt.Fprintln(os.Stderr, "Error: R2 credentials required (--r2-endpoint, --r2-bucket, --r2-access-key, --r2-secret-key)")
			os.Exit(1)
		}
	}

	runtimes := []string{*runtimeFlag}
	if *runtimeFlag == "all" {
		runtimes = []string{"node", "python", "ruby"}
	}

	// Initialize S3 client for R2
	var s3Client *s3.Client
	var existingKeys map[string]bool
	var builtFromSourceKeys map[string]bool

	if !*dryRun {
		var err error
		s3Client, err = createS3Client()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating S3 client: %v\n", err)
			os.Exit(1)
		}

		if *syncOnly {
			fmt.Println("Fetching existing files from R2...")
			existingKeys, err = listExistingKeys(s3Client)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error listing R2 contents: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Found %d existing files in R2\n", len(existingKeys))

			fmt.Println("Checking for built-from-source entries...")
			builtFromSourceKeys, err = listBuiltFromSourceKeys(s3Client, existingKeys)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error checking built-from-source keys: %v\n", err)
				os.Exit(1)
			}
			if len(builtFromSourceKeys) > 0 {
				fmt.Printf("Found %d built-from-source entries (will re-mirror from upstream)\n", len(builtFromSourceKeys))
			}
		}
	}

	// Collect all jobs from upstream sources
	var jobs []MirrorJob
	for _, rt := range runtimes {
		fmt.Printf("Discovering %s versions from upstream...\n", rt)
		rtJobs, err := fetchJobsFromUpstream(rt)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error fetching upstream versions for %s: %v\n", rt, err)
			os.Exit(1)
		}
		jobs = append(jobs, rtJobs...)
	}

	fmt.Printf("Total jobs to process: %d\n", len(jobs))

	if *dryRun {
		fmt.Println("\n[DRY RUN] Would mirror the following files:")
		withChecksum := 0
		withoutChecksum := 0
		for _, job := range jobs {
			checksumStatus := "(will generate checksum)"
			if job.UpstreamSHA256 != "" {
				checksumStatus = "(has upstream checksum)"
				withChecksum++
			} else {
				withoutChecksum++
			}
			fmt.Printf("  %s -> %s %s\n", job.URL, job.R2Key, checksumStatus)
		}
		fmt.Printf("\nTotal: %d files (%d with upstream checksum, %d will generate)\n",
			len(jobs), withChecksum, withoutChecksum)
		return
	}

	// Filter jobs if sync-only (check for .meta.json to determine if mirrored)
	if *syncOnly && existingKeys != nil {
		var filtered []MirrorJob
		for _, job := range jobs {
			if !existingKeys[job.MetaKey] {
				// Not yet mirrored
				filtered = append(filtered, job)
			} else if builtFromSourceKeys[job.MetaKey] {
				// Exists but was built from source — re-mirror from upstream
				filtered = append(filtered, job)
			}
		}
		skipped := len(jobs) - len(filtered)
		fmt.Printf("Skipping %d files already in R2, %d remaining\n", skipped, len(filtered))
		jobs = filtered
	}

	if len(jobs) == 0 {
		fmt.Println("No files to mirror")
		return
	}

	// Process jobs with worker pool
	stats := processJobs(s3Client, jobs)

	// Print summary
	fmt.Println("\n=== Mirror Summary ===")
	fmt.Printf("Total:              %d\n", stats.Total)
	fmt.Printf("Mirrored:           %d\n", stats.Mirrored)
	fmt.Printf("Skipped:            %d\n", stats.Skipped)
	fmt.Printf("Failed:             %d\n", stats.Failed)
	fmt.Printf("Bytes downloaded:   %d MB\n", stats.BytesDown/(1024*1024))
	fmt.Printf("Upstream checksums: %d\n", stats.UpstreamChecksum)
	fmt.Printf("Generated checksums: %d\n", stats.GeneratedChecksum)

	if stats.Failed > 0 {
		os.Exit(1)
	}
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

func listExistingKeys(client *s3.Client) (map[string]bool, error) {
	keys := make(map[string]bool)
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: r2Bucket,
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(context.Background())
		if err != nil {
			return nil, err
		}
		for _, obj := range page.Contents {
			keys[*obj.Key] = true
		}
	}

	return keys, nil
}

// listBuiltFromSourceKeys downloads .meta.json files that exist in R2 and returns
// the set of meta keys where source_url is "built-from-source". Only checks meta
// files that are in the existingKeys set to avoid unnecessary downloads.
func listBuiltFromSourceKeys(client *s3.Client, existingKeys map[string]bool) (map[string]bool, error) {
	builtFromSource := make(map[string]bool)

	// Collect all .meta.json keys from existingKeys
	var metaKeys []string
	for key := range existingKeys {
		if strings.HasSuffix(key, ".meta.json") {
			metaKeys = append(metaKeys, key)
		}
	}

	// Download and check each meta file
	for _, metaKey := range metaKeys {
		resp, err := client.GetObject(context.Background(), &s3.GetObjectInput{
			Bucket: r2Bucket,
			Key:    aws.String(metaKey),
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read %s: %v\n", metaKey, err)
			continue
		}

		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to read body of %s: %v\n", metaKey, err)
			continue
		}

		var meta BinaryMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to parse %s: %v\n", metaKey, err)
			continue
		}

		if meta.SourceURL == "built-from-source" {
			builtFromSource[metaKey] = true
		}
	}

	return builtFromSource, nil
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

func processJobs(client *s3.Client, jobs []MirrorJob) *Stats {
	stats := &Stats{Total: int64(len(jobs))}
	jobChan := make(chan MirrorJob, len(jobs))
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobChan {
				if err := mirrorFile(client, job, stats); err != nil {
					fmt.Fprintf(os.Stderr, "Error mirroring %s: %v\n", job.R2Key, err)
					atomic.AddInt64(&stats.Failed, 1)
				} else {
					atomic.AddInt64(&stats.Mirrored, 1)
				}
			}
		}()
	}

	// Queue jobs
	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)

	wg.Wait()
	return stats
}

func mirrorFile(client *s3.Client, job MirrorJob, stats *Stats) error {
	var lastErr error

	for attempt := 1; attempt <= *retries; attempt++ {
		if attempt > 1 {
			fmt.Printf("Retry %d/%d for %s\n", attempt, *retries, job.R2Key)
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		err := doMirror(client, job, stats)
		if err == nil {
			return nil
		}
		lastErr = err
	}

	return lastErr
}

func doMirror(client *s3.Client, job MirrorJob, stats *Stats) error {
	// Download file
	httpClient := &http.Client{Timeout: 10 * time.Minute}
	resp, err := httpClient.Get(job.URL)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Read body and calculate checksum
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read failed: %w", err)
	}

	atomic.AddInt64(&stats.BytesDown, int64(len(body)))

	// Calculate actual checksum
	hash := sha256.Sum256(body)
	actualChecksum := hex.EncodeToString(hash[:])

	// Determine checksum source and verify if upstream provided one
	var checksumSource string
	if job.UpstreamSHA256 != "" {
		if actualChecksum != job.UpstreamSHA256 {
			return fmt.Errorf("checksum mismatch: expected %s, got %s", job.UpstreamSHA256, actualChecksum)
		}
		checksumSource = "upstream"
		atomic.AddInt64(&stats.UpstreamChecksum, 1)
	} else {
		checksumSource = "dtvem"
		atomic.AddInt64(&stats.GeneratedChecksum, 1)
	}

	// Determine content type
	contentType := "application/octet-stream"
	if strings.HasSuffix(job.R2Key, ".tar.gz") {
		contentType = "application/gzip"
	} else if strings.HasSuffix(job.R2Key, ".zip") {
		contentType = "application/zip"
	} else if strings.HasSuffix(job.R2Key, ".tar.xz") {
		contentType = "application/x-xz"
	}

	// Upload binary to R2
	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       r2Bucket,
		Key:          aws.String(job.R2Key),
		Body:         bytes.NewReader(body),
		ContentType:  aws.String(contentType),
		CacheControl: aws.String("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return fmt.Errorf("upload binary failed: %w", err)
	}

	// Create and upload metadata
	meta := BinaryMeta{
		SHA256:       actualChecksum,
		SHA256Source: checksumSource,
		SourceURL:    job.URL,
		MirroredAt:   time.Now().UTC().Format(time.RFC3339),
		Size:         int64(len(body)),
	}

	metaJSON, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata failed: %w", err)
	}

	_, err = client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:       r2Bucket,
		Key:          aws.String(job.MetaKey),
		Body:         bytes.NewReader(metaJSON),
		ContentType:  aws.String("application/json"),
		CacheControl: aws.String("public, max-age=300"), // Short cache for metadata
	})
	if err != nil {
		return fmt.Errorf("upload metadata failed: %w", err)
	}

	fmt.Printf("Mirrored: %s (%d bytes, checksum: %s)\n", job.R2Key, len(body), checksumSource)
	return nil
}
