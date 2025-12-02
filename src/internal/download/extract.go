package download

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractZip extracts a zip archive to a destination directory
func ExtractZip(zipPath, destDir string) error {
	// Open zip file
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Extract each file
	for _, file := range reader.File {
		if err := extractZipFile(file, destDir); err != nil {
			return fmt.Errorf("failed to extract %s: %w", file.Name, err)
		}
	}

	return nil
}

func extractZipFile(file *zip.File, destDir string) error {
	// Build destination path
	destPath := filepath.Join(destDir, file.Name)

	// Check for ZipSlip vulnerability
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path: %s", file.Name)
	}

	if file.FileInfo().IsDir() {
		// Create directory
		return os.MkdirAll(destPath, file.Mode())
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Open source file
	srcFile, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	// Create destination file
	destFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, file.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = destFile.Close() }()

	// Copy data
	_, err = io.Copy(destFile, srcFile)
	return err
}

// ExtractTarGz extracts a tar.gz archive to a destination directory
func ExtractTarGz(tarGzPath, destDir string) error {
	// Open tar.gz file
	file, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer func() { _ = gzReader.Close() }()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Extract each file
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if err := extractTarFile(header, tarReader, destDir); err != nil {
			return fmt.Errorf("failed to extract %s: %w", header.Name, err)
		}
	}

	return nil
}

func extractTarFile(header *tar.Header, reader io.Reader, destDir string) error {
	// Build destination path
	destPath := filepath.Join(destDir, header.Name)

	// Check for ZipSlip vulnerability
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path: %s", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		// Create directory
		return os.MkdirAll(destPath, os.FileMode(header.Mode))

	case tar.TypeReg:
		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Create file
		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		defer func() { _ = outFile.Close() }()

		// Copy data
		_, err = io.Copy(outFile, reader)
		return err

	case tar.TypeSymlink:
		// Create parent directory
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		// Create symlink
		return os.Symlink(header.Linkname, destPath)

	default:
		// Skip other types
		return nil
	}
}

// StripTopLevelDir removes the top-level directory from an extraction
// This is useful when archives contain a single top-level directory
// (e.g., node-v18.16.0/ containing bin/, lib/, etc.)
func StripTopLevelDir(extractDir string) error {
	// Read directory contents
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return err
	}

	// Check if there's exactly one entry and it's a directory
	if len(entries) != 1 || !entries[0].IsDir() {
		return nil // Nothing to strip
	}

	// Create a temporary directory
	tempDir := extractDir + ".tmp"
	if err := os.Rename(extractDir, tempDir); err != nil {
		return err
	}

	// Move the contents of the top-level directory to the extraction directory
	if err := os.Rename(filepath.Join(tempDir, entries[0].Name()), extractDir); err != nil {
		// Try to recover
		_ = os.Rename(tempDir, extractDir)
		return err
	}

	// Remove the temporary directory
	return os.RemoveAll(tempDir)
}
