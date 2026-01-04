package download

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/schollz/progressbar/v3"
)

// ErrChecksumMismatch is returned when the downloaded file's checksum doesn't match.
type ErrChecksumMismatch struct {
	Expected string
	Actual   string
}

func (e *ErrChecksumMismatch) Error() string {
	return fmt.Sprintf("checksum mismatch: expected %s, got %s", e.Expected, e.Actual)
}

// FileVerified downloads a file from a URL and verifies its SHA256 checksum.
// If the checksum doesn't match, the file is deleted and an error is returned.
func FileVerified(url, destPath, expectedSHA256 string) error {
	ui.Debug("Starting verified download: %s", url)
	ui.Debug("Destination: %s", destPath)
	ui.Debug("Expected SHA256: %s", expectedSHA256)

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

	// Create SHA256 hasher
	hasher := sha256.New()

	// Copy data with progress bar and hashing
	_, err = io.Copy(io.MultiWriter(out, bar, hasher), resp.Body)
	if err != nil {
		ui.Debug("Download failed: %v", err)
		_ = os.Remove(destPath) // Clean up partial download
		return err
	}

	fmt.Println() // New line after progress bar

	// Verify checksum
	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))
	ui.Debug("Actual SHA256: %s", actualSHA256)

	// Normalize both checksums to lowercase for comparison
	expectedNorm := strings.ToLower(strings.TrimSpace(expectedSHA256))
	actualNorm := strings.ToLower(actualSHA256)

	if actualNorm != expectedNorm {
		ui.Debug("Checksum mismatch! Removing downloaded file.")
		_ = os.Remove(destPath) // Remove the file with bad checksum
		return &ErrChecksumMismatch{
			Expected: expectedSHA256,
			Actual:   actualSHA256,
		}
	}

	ui.Debug("Checksum verified successfully")
	ui.Debug("Download complete: %s", destPath)
	return nil
}

// VerifyFile checks if an existing file matches the expected SHA256 checksum.
func VerifyFile(filePath, expectedSHA256 string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return err
	}

	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))

	// Normalize both checksums to lowercase for comparison
	expectedNorm := strings.ToLower(strings.TrimSpace(expectedSHA256))
	actualNorm := strings.ToLower(actualSHA256)

	if actualNorm != expectedNorm {
		return &ErrChecksumMismatch{
			Expected: expectedSHA256,
			Actual:   actualSHA256,
		}
	}

	return nil
}

// ComputeSHA256 computes the SHA256 checksum of a file.
func ComputeSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}
