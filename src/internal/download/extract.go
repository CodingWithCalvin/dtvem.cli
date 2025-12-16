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

	"github.com/bodgit/sevenzip"
	"github.com/dtvem/dtvem/src/internal/ui"
)

// archiveFile is an interface for files within an archive (zip or 7z)
type archiveFile interface {
	Open() (io.ReadCloser, error)
	Name() string
	Mode() os.FileMode
	IsDir() bool
}

// zipFileAdapter wraps zip.File to implement archiveFile
type zipFileAdapter struct{ *zip.File }

func (z *zipFileAdapter) Name() string      { return z.File.Name }
func (z *zipFileAdapter) Mode() os.FileMode { return z.File.Mode() }
func (z *zipFileAdapter) IsDir() bool       { return z.File.FileInfo().IsDir() }

// sevenzipFileAdapter wraps sevenzip.File to implement archiveFile
type sevenzipFileAdapter struct{ *sevenzip.File }

func (s *sevenzipFileAdapter) Name() string      { return s.File.Name }
func (s *sevenzipFileAdapter) Mode() os.FileMode { return s.File.Mode() }
func (s *sevenzipFileAdapter) IsDir() bool       { return s.File.FileInfo().IsDir() }

// ExtractZip extracts a zip archive to a destination directory
func ExtractZip(zipPath, destDir string) error {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		ui.Debug("Failed to open ZIP: %v", err)
		return fmt.Errorf("failed to open archive: %w (file: %s)", err, zipPath)
	}
	defer func() { _ = reader.Close() }()

	files := make([]archiveFile, len(reader.File))
	for i, f := range reader.File {
		files[i] = &zipFileAdapter{f}
	}
	return extractArchive("ZIP", zipPath, destDir, files)
}

// Extract7z extracts a 7z archive to a destination directory
func Extract7z(szPath, destDir string) error {
	reader, err := sevenzip.OpenReader(szPath)
	if err != nil {
		ui.Debug("Failed to open 7z: %v", err)
		return fmt.Errorf("failed to open archive: %w (file: %s)", err, szPath)
	}
	defer func() { _ = reader.Close() }()

	files := make([]archiveFile, len(reader.File))
	for i, f := range reader.File {
		files[i] = &sevenzipFileAdapter{f}
	}
	return extractArchive("7z", szPath, destDir, files)
}

// extractArchive is a generic extractor for zip-like archives
func extractArchive(archiveType, archivePath, destDir string, files []archiveFile) error {
	ui.Debug("Extracting %s: %s", archiveType, archivePath)
	ui.Debug("Destination: %s", destDir)
	ui.Debug("%s contains %d files", archiveType, len(files))

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	// Extract each file
	for _, file := range files {
		if err := extractArchiveFile(file, destDir); err != nil {
			return fmt.Errorf("failed to extract %s: %w", file.Name(), err)
		}
	}

	ui.Debug("%s extraction complete", archiveType)
	return nil
}

// extractArchiveFile extracts a single file from an archive (zip or 7z)
func extractArchiveFile(file archiveFile, destDir string) error {
	// Build destination path
	destPath := filepath.Join(destDir, file.Name())

	// Check for ZipSlip vulnerability
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path: %s", file.Name())
	}

	if file.IsDir() {
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

	_, err = io.Copy(destFile, srcFile)
	return err
}

// ExtractTarGz extracts a tar.gz archive to a destination directory
func ExtractTarGz(tarGzPath, destDir string) error {
	ui.Debug("Extracting tar.gz: %s", tarGzPath)
	ui.Debug("Destination: %s", destDir)

	file, err := os.Open(tarGzPath)
	if err != nil {
		ui.Debug("Failed to open tar.gz: %v", err)
		return fmt.Errorf("failed to open archive: %w (file: %s)", err, tarGzPath)
	}
	defer func() { _ = file.Close() }()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		ui.Debug("Failed to create gzip reader: %v", err)
		return fmt.Errorf("invalid gzip archive: %w (file: %s)", err, tarGzPath)
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return err
	}

	fileCount := 0
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
		fileCount++
	}

	ui.Debug("tar.gz extraction complete: %d files extracted", fileCount)
	return nil
}

func extractTarFile(header *tar.Header, reader io.Reader, destDir string) error {
	destPath := filepath.Join(destDir, header.Name)

	// Check for ZipSlip vulnerability
	if !strings.HasPrefix(destPath, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("illegal file path: %s", header.Name)
	}

	switch header.Typeflag {
	case tar.TypeDir:
		return os.MkdirAll(destPath, os.FileMode(header.Mode))

	case tar.TypeReg:
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
		if err != nil {
			return err
		}
		defer func() { _ = outFile.Close() }()

		_, err = io.Copy(outFile, reader)
		return err

	case tar.TypeSymlink:
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return err
		}
		return os.Symlink(header.Linkname, destPath)

	default:
		return nil
	}
}

// StripTopLevelDir removes the top-level directory from an extraction
// This is useful when archives contain a single top-level directory
// (e.g., node-v18.16.0/ containing bin/, lib/, etc.)
func StripTopLevelDir(extractDir string) error {
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return err
	}

	if len(entries) != 1 || !entries[0].IsDir() {
		return nil
	}

	tempDir := extractDir + ".tmp"
	if err := os.Rename(extractDir, tempDir); err != nil {
		return err
	}

	if err := os.Rename(filepath.Join(tempDir, entries[0].Name()), extractDir); err != nil {
		_ = os.Rename(tempDir, extractDir)
		return err
	}

	return os.RemoveAll(tempDir)
}
