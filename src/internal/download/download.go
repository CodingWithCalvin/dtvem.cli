// Package download provides utilities for downloading and extracting runtime archives
package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/schollz/progressbar/v3"
)

// File downloads a file from a URL to a destination path with a progress bar
func File(url, destPath string) error {
	ui.Debug("Starting download: %s", url)
	ui.Debug("Destination: %s", destPath)

	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Create the destination file
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	// Make HTTP request
	ui.Debug("Making HTTP GET request...")
	resp, err := http.Get(url)
	if err != nil {
		ui.Debug("HTTP request failed: %v", err)
		return fmt.Errorf("failed to connect: %w (URL: %s)", err, url)
	}
	defer func() { _ = resp.Body.Close() }()

	ui.Debug("HTTP response: %s", resp.Status)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed (HTTP %s): %s", resp.Status, url)
	}

	// Get file size for progress bar
	size := resp.ContentLength
	ui.Debug("Content-Length: %d bytes", size)

	// Create progress bar
	bar := progressbar.DefaultBytes(
		size,
		"Downloading",
	)

	// Copy data with progress bar
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		ui.Debug("Download failed: %v", err)
		return err
	}

	fmt.Println() // New line after progress bar
	ui.Debug("Download complete: %s", destPath)
	return nil
}

// FileWithProgress downloads a file and reports progress
func FileWithProgress(url, destPath string, progress func(current, total int64)) error {
	// Create destination directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Create the destination file
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	// Make HTTP request
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to connect: %w (URL: %s)", err, url)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed (HTTP %s): %s", resp.Status, url)
	}

	// Get total size
	totalSize := resp.ContentLength

	// Create progress reader
	reader := &progressReader{
		reader:   resp.Body,
		progress: progress,
		total:    totalSize,
	}

	// Copy data
	_, err = io.Copy(out, reader)
	if err != nil {
		return err
	}

	return nil
}

// progressReader wraps an io.Reader and reports progress
type progressReader struct {
	reader   io.Reader
	progress func(current, total int64)
	current  int64
	total    int64
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.current += int64(n)

	if pr.progress != nil {
		pr.progress(pr.current, pr.total)
	}

	return n, err
}
