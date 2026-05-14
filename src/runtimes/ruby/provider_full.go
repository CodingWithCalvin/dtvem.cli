//go:build !shim

// This file holds the "full half" of the Ruby provider: methods that
// install, list, migrate, and otherwise touch the network or extract archives.
// Excluded from shim builds so the shim binary doesn't link net/http, sevenzip,
// klauspost/compress, brotli, lz4, xz, embedded manifests, etc.
package ruby

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/download"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/manifest"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
)

// Install downloads and installs a specific version.
func (p *Provider) Install(version string) error {
	ui.Debug("Starting Ruby installation for version %s", version)

	if err := config.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create dtvem directories: %w", err)
	}

	if installed, _ := p.IsInstalled(version); installed {
		return fmt.Errorf("Ruby %s is already installed", version)
	}

	ui.Header("Installing Ruby v%s...", version)

	downloadURL, archiveName, err := p.getDownloadURL(version)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}
	ui.Debug("Download URL: %s", downloadURL)
	ui.Debug("Archive name: %s", archiveName)

	extractDir, cleanup, err := p.downloadAndExtract(version, downloadURL, archiveName)
	if err != nil {
		return err
	}
	defer cleanup()

	sourceDir := p.determineSourceDir(extractDir)
	ui.Debug("Source directory: %s", sourceDir)

	installPath := config.RuntimeVersionPath("ruby", version)
	ui.Debug("Install path: %s", installPath)

	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	ui.Debug("Moving files from %s to %s", sourceDir, installPath)
	if err := os.Rename(sourceDir, installPath); err != nil {
		return fmt.Errorf("failed to move to install location: %w", err)
	}

	shimSpinner := ui.NewSpinner("Creating shims...")
	shimSpinner.Start()
	if err := p.createShims(version); err != nil {
		shimSpinner.Error("Failed to create shims")
		return fmt.Errorf("failed to create shims: %w", err)
	}
	shimSpinner.Success("Shims created")

	ui.Success("Ruby v%s installed successfully", version)
	ui.Info("Location: %s", installPath)

	return nil
}

// downloadAndExtract downloads and extracts the Ruby archive.
func (p *Provider) downloadAndExtract(version, downloadURL, archiveName string) (extractDir string, cleanup func(), err error) {
	ui.Progress("Downloading from %s", downloadURL)

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("dtvem-ruby-%s", version))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanupFunc := func() { _ = os.RemoveAll(tempDir) }

	archivePath := filepath.Join(tempDir, archiveName)
	if err := download.File(downloadURL, archivePath); err != nil {
		cleanupFunc()
		return "", nil, fmt.Errorf("failed to download: %w", err)
	}

	// Handle .exe installer specially (Windows RubyInstaller)
	if strings.HasSuffix(archiveName, ".exe") {
		return p.runWindowsInstaller(version, archivePath, tempDir, cleanupFunc)
	}

	extractDir = filepath.Join(tempDir, "extracted")
	spinner := ui.NewSpinner("Extracting archive...")
	spinner.Start()

	var extractErr error
	if strings.HasSuffix(archiveName, ".zip") {
		extractErr = download.ExtractZip(archivePath, extractDir)
	} else if strings.HasSuffix(archiveName, ".tar.gz") || strings.HasSuffix(archiveName, ".tar.xz") {
		extractErr = download.ExtractTarGz(archivePath, extractDir)
	} else if strings.HasSuffix(archiveName, ".7z") {
		extractErr = download.Extract7z(archivePath, extractDir)
	} else {
		extractErr = fmt.Errorf("unsupported archive format: %s", archiveName)
	}

	if extractErr != nil {
		spinner.Error("Extraction failed")
		cleanupFunc()
		return "", nil, fmt.Errorf("failed to extract: %w", extractErr)
	}

	spinner.Success("Extraction complete")
	return extractDir, cleanupFunc, nil
}

// runWindowsInstaller runs the RubyInstaller .exe in silent mode.
func (p *Provider) runWindowsInstaller(version, installerPath, tempDir string, cleanupFunc func()) (string, func(), error) {
	extractDir := filepath.Join(tempDir, "installed")

	spinner := ui.NewSpinner("Running installer (silent mode)...")
	spinner.Start()

	// /VERYSILENT, /SUPPRESSMSGBOXES, /NORESTART, /CURRENTUSER (no admin), /DIR=...,
	// /TASKS="" (no PATH modification, no file associations).
	cmd := exec.Command(installerPath,
		"/VERYSILENT",
		"/SUPPRESSMSGBOXES",
		"/NORESTART",
		"/CURRENTUSER",
		"/DIR="+extractDir,
		"/TASKS=",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		spinner.Error("Installation failed")
		cleanupFunc()
		ui.Debug("Installer output: %s", string(output))
		return "", nil, fmt.Errorf("installer failed: %w", err)
	}

	spinner.Success("Installation complete")
	return extractDir, cleanupFunc, nil
}

// determineSourceDir determines the source directory from extracted archive.
func (p *Provider) determineSourceDir(extractDir string) string {
	// Check for ruby-build format (ruby/ subdirectory)
	rubySubdir := filepath.Join(extractDir, "ruby")
	if _, err := os.Stat(rubySubdir); err == nil {
		return rubySubdir
	}

	// Check for RubyInstaller format on Windows (rubyXX-version directory)
	entries, err := os.ReadDir(extractDir)
	if err == nil && len(entries) == 1 && entries[0].IsDir() {
		return filepath.Join(extractDir, entries[0].Name())
	}

	return extractDir
}

// getDownloadURL returns the download URL and archive name for a given version.
func (p *Provider) getDownloadURL(version string) (string, string, error) {
	m, err := manifest.DefaultSource().GetManifest("ruby")
	if err != nil {
		return "", "", fmt.Errorf("failed to load manifest: %w", err)
	}

	platform := manifest.CurrentPlatform()
	dl := m.GetDownload(version, platform)
	if dl == nil {
		return "", "", fmt.Errorf("Ruby %s is not available for %s", version, platform)
	}

	archiveName := filepath.Base(dl.URL)

	return dl.URL, archiveName, nil
}

// createShims creates shims for Ruby executables and registers them in the
// shim-map cache so subsequent shim invocations resolve via O(1) lookup rather
// than falling back to the provider registry. The version is recorded in the
// cache so the shim can detect when an active runtime version is one that
// does not provide a given executable.
//
// The shim list is derived from disk (the same scan reshim uses), not from
// the provider's static Shims() declaration, so install and reshim stay in
// sync and only register executables that actually exist for this version.
func (p *Provider) createShims(version string) error {
	manager, err := shim.NewManager()
	if err != nil {
		return err
	}

	versionDir := config.RuntimeVersionPath("ruby", version)
	shimNames := shim.DiscoverShimsForVersion(versionDir)
	if len(shimNames) == 0 {
		return fmt.Errorf("no executables found in %s", versionDir)
	}

	return manager.CreateShimsForRuntime("ruby", version, shimNames)
}

// Uninstall removes an installed version.
func (p *Provider) Uninstall(version string) error {
	return fmt.Errorf("not yet implemented")
}

// ListInstalled returns all installed Ruby versions.
func (p *Provider) ListInstalled() ([]runtime.InstalledVersion, error) {
	paths := config.DefaultPaths()
	rubyVersionsDir := filepath.Join(paths.Versions, "ruby")

	if _, err := os.Stat(rubyVersionsDir); os.IsNotExist(err) {
		return []runtime.InstalledVersion{}, nil
	}

	entries, err := os.ReadDir(rubyVersionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	versions := make([]runtime.InstalledVersion, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, runtime.InstalledVersion{
				Version:     runtime.NewVersion(entry.Name()),
				InstallPath: filepath.Join(rubyVersionsDir, entry.Name()),
				IsGlobal:    false,
			})
		}
	}

	return versions, nil
}

// ListAvailable returns all available Ruby versions.
func (p *Provider) ListAvailable() ([]runtime.AvailableVersion, error) {
	m, err := manifest.DefaultSource().GetManifest("ruby")
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	platform := manifest.CurrentPlatform()
	versionStrings := m.ListAvailableVersions(platform)

	versions := make([]runtime.AvailableVersion, 0, len(versionStrings))
	for _, v := range versionStrings {
		versions = append(versions, runtime.AvailableVersion{
			Version: runtime.NewVersion(v),
		})
	}

	runtime.SortVersionsDesc(versions)

	return versions, nil
}

// GlobalVersion returns the globally configured version.
func (p *Provider) GlobalVersion() (string, error) {
	return config.GlobalVersion("ruby")
}

// SetGlobalVersion sets the global default version.
func (p *Provider) SetGlobalVersion(version string) error {
	return config.SetGlobalVersion("ruby", version)
}

// LocalVersion returns the locally configured version.
func (p *Provider) LocalVersion() (string, error) {
	version, err := config.ResolveVersion("ruby")
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetLocalVersion sets the local version for current directory.
func (p *Provider) SetLocalVersion(version string) error {
	return config.SetLocalVersion("ruby", version)
}

// CurrentVersion returns the currently active version.
func (p *Provider) CurrentVersion() (string, error) {
	return config.ResolveVersion("ruby")
}

// DetectInstalled scans the system for existing Ruby installations.
// Detection is handled by migration providers in src/migrations/; this
// method returns empty to avoid duplicate code.
func (p *Provider) DetectInstalled() ([]runtime.DetectedVersion, error) {
	return []runtime.DetectedVersion{}, nil
}

// GlobalPackages detects globally installed gems.
func (p *Provider) GlobalPackages(installPath string) ([]string, error) {
	gemPath := findGemInInstall(installPath)
	if gemPath == "" {
		return nil, fmt.Errorf("gem not found in installation")
	}

	cmd := exec.Command(gemPath, "list", "--no-details")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list gems: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	packages := make([]string, 0, len(lines))

	skipGems := map[string]bool{
		"bundler":         true,
		"rake":            true,
		"rdoc":            true,
		"irb":             true,
		"reline":          true,
		"io-console":      true,
		"psych":           true,
		"json":            true,
		"bigdecimal":      true,
		"date":            true,
		"delegate":        true,
		"did_you_mean":    true,
		"error_highlight": true,
		"fileutils":       true,
		"getoptlong":      true,
		"minitest":        true,
		"net-ftp":         true,
		"net-http":        true,
		"net-imap":        true,
		"net-pop":         true,
		"net-protocol":    true,
		"net-smtp":        true,
		"observer":        true,
		"open-uri":        true,
		"open3":           true,
		"optparse":        true,
		"ostruct":         true,
		"power_assert":    true,
		"pp":              true,
		"prettyprint":     true,
		"pstore":          true,
		"racc":            true,
		"readline":        true,
		"resolv":          true,
		"resolv-replace":  true,
		"rinda":           true,
		"rss":             true,
		"securerandom":    true,
		"set":             true,
		"shellwords":      true,
		"singleton":       true,
		"stringio":        true,
		"strscan":         true,
		"syslog":          true,
		"tempfile":        true,
		"test-unit":       true,
		"time":            true,
		"timeout":         true,
		"tmpdir":          true,
		"tsort":           true,
		"un":              true,
		"uri":             true,
		"weakref":         true,
		"webrick":         true,
		"yaml":            true,
		"zlib":            true,
		"abbrev":          true,
		"base64":          true,
		"benchmark":       true,
		"cgi":             true,
		"csv":             true,
		"debug":           true,
		"digest":          true,
		"drb":             true,
		"english":         true,
		"erb":             true,
		"etc":             true,
		"fcntl":           true,
		"fiddle":          true,
		"forwardable":     true,
		"ipaddr":          true,
		"logger":          true,
		"matrix":          true,
		"mutex_m":         true,
		"nkf":             true,
		"openssl":         true,
		"pathname":        true,
		"prime":           true,
		"readline-ext":    true,
		"rexml":           true,
		"rubygems-update": true,
	}

	gemRegex := regexp.MustCompile(`^([a-zA-Z0-9_-]+)`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		matches := gemRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			gemName := matches[1]
			if !skipGems[gemName] {
				packages = append(packages, gemName)
			}
		}
	}

	return packages, nil
}

// InstallGlobalPackages reinstalls global gems to a specific version.
func (p *Provider) InstallGlobalPackages(version string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	execPath, err := p.ExecutablePath(version)
	if err != nil {
		return err
	}

	installDir := filepath.Dir(execPath)
	gemPath := findGemInInstall(installDir)
	if gemPath == "" {
		return fmt.Errorf("gem not found in installation")
	}

	args := append([]string{"install"}, packages...)
	cmd := exec.Command(gemPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gem install failed: %w\n%s", err, string(output))
	}

	return nil
}

// ManualPackageInstallCommand returns the command for manually installing gems.
func (p *Provider) ManualPackageInstallCommand(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	return fmt.Sprintf("gem install %s", strings.Join(packages, " "))
}

// findGemInInstall finds the gem executable in an installation directory.
func findGemInInstall(installDir string) string {
	searchPaths := []string{
		installDir,
		filepath.Join(installDir, "bin"),
	}

	if goruntime.GOOS == constants.OSWindows {
		for _, searchPath := range searchPaths {
			cmdPath := filepath.Join(searchPath, "gem.cmd")
			if _, err := os.Stat(cmdPath); err == nil {
				return cmdPath
			}
			batPath := filepath.Join(searchPath, "gem.bat")
			if _, err := os.Stat(batPath); err == nil {
				return batPath
			}
			exePath := filepath.Join(searchPath, "gem.exe")
			if _, err := os.Stat(exePath); err == nil {
				return exePath
			}
		}
	} else {
		for _, searchPath := range searchPaths {
			execPath := filepath.Join(searchPath, "gem")
			if _, err := os.Stat(execPath); err == nil {
				return execPath
			}
		}
	}

	return ""
}
