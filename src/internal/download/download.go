// Package download provides utilities for downloading and extracting runtime archives
package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
)

// File downloads a file from a URL to a destination path with a progress bar
func File(url, destPath string) error {
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
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	// Get file size for progress bar
	size := resp.ContentLength

	// Create progress bar
	bar := progressbar.DefaultBytes(
		size,
		"Downloading",
	)

	// Copy data with progress bar
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		return err
	}

	fmt.Println() // New line after progress bar
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
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
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
