// Package ruby implements the Ruby runtime provider for dtvem
package ruby

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strings"
	"time"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/download"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/shim"
	"github.com/dtvem/dtvem/src/internal/ui"
)

// Provider implements the runtime.Provider interface for Ruby
type Provider struct {
	// Configuration and state will go here
}

// NewProvider creates a new Ruby runtime provider
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the runtime name
func (p *Provider) Name() string {
	return "ruby"
}

// DisplayName returns the human-readable name
func (p *Provider) DisplayName() string {
	return "Ruby"
}

// Shims returns the list of shim executables for Ruby
func (p *Provider) Shims() []string {
	return []string{"ruby", "gem", "irb", "bundle", "rake", "rdoc", "ri"}
}

// Install downloads and installs a specific version
func (p *Provider) Install(version string) error {
	ui.Debug("Starting Ruby installation for version %s", version)

	// Ensure dtvem directories exist
	if err := config.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create dtvem directories: %w", err)
	}

	// Check if already installed
	if installed, _ := p.IsInstalled(version); installed {
		return fmt.Errorf("Ruby %s is already installed", version)
	}

	ui.Header("Installing Ruby v%s...", version)

	// Get platform-specific download URL
	downloadURL, archiveName, err := p.getDownloadURL(version)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}
	ui.Debug("Download URL: %s", downloadURL)
	ui.Debug("Archive name: %s", archiveName)

	// Download and extract
	extractDir, cleanup, err := p.downloadAndExtract(version, downloadURL, archiveName)
	if err != nil {
		return err
	}
	defer cleanup()

	// Determine source directory
	sourceDir := p.determineSourceDir(extractDir)
	ui.Debug("Source directory: %s", sourceDir)

	// Get install path and move files
	installPath := config.RuntimeVersionPath("ruby", version)
	ui.Debug("Install path: %s", installPath)

	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	ui.Debug("Moving files from %s to %s", sourceDir, installPath)
	if err := os.Rename(sourceDir, installPath); err != nil {
		return fmt.Errorf("failed to move to install location: %w", err)
	}

	// Create shims
	shimSpinner := ui.NewSpinner("Creating shims...")
	shimSpinner.Start()
	if err := p.createShims(); err != nil {
		shimSpinner.Error("Failed to create shims")
		return fmt.Errorf("failed to create shims: %w", err)
	}
	shimSpinner.Success("Shims created")

	ui.Success("Ruby v%s installed successfully", version)
	ui.Info("Location: %s", installPath)

	return nil
}

// downloadAndExtract downloads and extracts the Ruby archive
func (p *Provider) downloadAndExtract(version, downloadURL, archiveName string) (extractDir string, cleanup func(), err error) {
	ui.Progress("Downloading from %s", downloadURL)

	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("dtvem-ruby-%s", version))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanupFunc := func() { _ = os.RemoveAll(tempDir) }

	// Download archive
	archivePath := filepath.Join(tempDir, archiveName)
	if err := download.File(downloadURL, archivePath); err != nil {
		cleanupFunc()
		return "", nil, fmt.Errorf("failed to download: %w", err)
	}

	// Handle .exe installer specially (Windows RubyInstaller)
	if strings.HasSuffix(archiveName, ".exe") {
		return p.runWindowsInstaller(version, archivePath, tempDir, cleanupFunc)
	}

	// Extract archive
	extractDir = filepath.Join(tempDir, "extracted")
	spinner := ui.NewSpinner("Extracting archive...")
	spinner.Start()

	var extractErr error
	if strings.HasSuffix(archiveName, ".zip") {
		extractErr = download.ExtractZip(archivePath, extractDir)
	} else if strings.HasSuffix(archiveName, ".tar.gz") || strings.HasSuffix(archiveName, ".tar.xz") {
		extractErr = download.ExtractTarGz(archivePath, extractDir)
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

// runWindowsInstaller runs the RubyInstaller .exe in silent mode
func (p *Provider) runWindowsInstaller(version, installerPath, tempDir string, cleanupFunc func()) (string, func(), error) {
	// Install to a temporary location, then we'll move it
	extractDir := filepath.Join(tempDir, "installed")

	spinner := ui.NewSpinner("Running installer (silent mode)...")
	spinner.Start()

	// Run the installer in very silent mode with:
	// - /VERYSILENT: no UI at all
	// - /SUPPRESSMSGBOXES: suppress message boxes
	// - /NORESTART: don't restart
	// - /CURRENTUSER: per-user install (no admin required)
	// - /DIR=...: custom install directory
	// - /TASKS="": no additional tasks (no PATH modification, no file associations)
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

// determineSourceDir determines the source directory from extracted archive
func (p *Provider) determineSourceDir(extractDir string) string {
	// Check for ruby-build format (ruby/ subdirectory)
	rubySubdir := filepath.Join(extractDir, "ruby")
	if _, err := os.Stat(rubySubdir); err == nil {
		return rubySubdir
	}

	// Check for RubyInstaller format on Windows (rubyXX-version directory)
	entries, err := os.ReadDir(extractDir)
	if err == nil && len(entries) == 1 && entries[0].IsDir() {
		// Single directory - use it
		return filepath.Join(extractDir, entries[0].Name())
	}

	// Fallback: use extractDir if nothing else matches
	return extractDir
}

// getDownloadURL returns the download URL and archive name for a given version
func (p *Provider) getDownloadURL(version string) (string, string, error) {
	platform := goruntime.GOOS
	arch := goruntime.GOARCH

	switch platform {
	case constants.OSWindows:
		return p.getRubyInstallerURL(version, arch)
	case constants.OSDarwin, constants.OSLinux:
		return p.getRubyBuildURL(version, platform, arch)
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", platform)
	}
}

// getRubyInstallerURL constructs URL for RubyInstaller on Windows
func (p *Provider) getRubyInstallerURL(version, arch string) (string, string, error) {
	// RubyInstaller provides prebuilt Windows binaries
	// We use the .exe installer and run it in silent mode with custom directory

	// Map Go arch to RubyInstaller arch
	var rubyArch string
	if arch == constants.ArchAMD64 {
		rubyArch = "x64"
	} else if arch == constants.Arch386 {
		rubyArch = "x86"
	} else {
		return "", "", fmt.Errorf("unsupported Windows architecture: %s", arch)
	}

	// RubyInstaller uses a patch version like -1, -2, etc.
	archiveName := fmt.Sprintf("rubyinstaller-%s-1-%s.exe", version, rubyArch)
	downloadURL := fmt.Sprintf("https://github.com/oneclick/rubyinstaller2/releases/download/RubyInstaller-%s-1/%s",
		version, archiveName)

	return downloadURL, archiveName, nil
}

// getRubyBuildURL constructs URL for ruby-builder releases
func (p *Provider) getRubyBuildURL(version, platform, arch string) (string, string, error) {
	// Use ruby-builder releases from ruby/ruby-builder (GitHub Actions)
	// These provide prebuilt Ruby binaries

	var rbsArch string
	switch arch {
	case constants.ArchAMD64:
		rbsArch = "x64"
	case constants.ArchARM64:
		rbsArch = "arm64"
	default:
		return "", "", fmt.Errorf("unsupported architecture for %s: %s", platform, arch)
	}

	var rbsPlatform string
	switch platform {
	case constants.OSDarwin:
		// Format: ruby-3.4.7-macos-arm64.tar.gz or ruby-3.4.7-macos-13-arm64.tar.gz
		rbsPlatform = "macos"
	case constants.OSLinux:
		// Format: ruby-3.4.7-ubuntu-22.04-x64.tar.gz
		rbsPlatform = "ubuntu-22.04"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", platform)
	}

	// Format: ruby-3.4.7-macos-arm64.tar.gz or ruby-3.4.7-ubuntu-22.04-x64.tar.gz
	archiveName := fmt.Sprintf("ruby-%s-%s-%s.tar.gz", version, rbsPlatform, rbsArch)

	// Download from ruby-builder releases using toolcache tag
	downloadURL := fmt.Sprintf("https://github.com/ruby/ruby-builder/releases/download/toolcache/%s", archiveName)

	return downloadURL, archiveName, nil
}

// createShims creates shims for Ruby executables
func (p *Provider) createShims() error {
	manager, err := shim.NewManager()
	if err != nil {
		return err
	}

	// Get the list of shims for Ruby
	shimNames := shim.RuntimeShims("ruby")

	// Create each shim
	return manager.CreateShims(shimNames)
}

// Uninstall removes an installed version
func (p *Provider) Uninstall(version string) error {
	return fmt.Errorf("not yet implemented")
}

// ListInstalled returns all installed Ruby versions
func (p *Provider) ListInstalled() ([]runtime.InstalledVersion, error) {
	paths := config.DefaultPaths()
	rubyVersionsDir := filepath.Join(paths.Versions, "ruby")

	// Check if directory exists
	if _, err := os.Stat(rubyVersionsDir); os.IsNotExist(err) {
		return []runtime.InstalledVersion{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(rubyVersionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	// Build list of installed versions
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

// ghRelease represents a GitHub release from the API
type ghRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
	} `json:"assets"`
}

// ListAvailable returns all available Ruby versions
func (p *Provider) ListAvailable() ([]runtime.AvailableVersion, error) {
	// On Windows, use RubyInstaller releases
	// On macOS/Linux, use ruby-builder releases
	if goruntime.GOOS == constants.OSWindows {
		return p.listAvailableWindows()
	}
	return p.listAvailableUnix()
}

// listAvailableWindows fetches available versions from RubyInstaller
func (p *Provider) listAvailableWindows() ([]runtime.AvailableVersion, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get RubyInstaller releases
	url := "https://api.github.com/repos/oneclick/rubyinstaller2/releases"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dtvem")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch version list: HTTP %d", resp.StatusCode)
	}

	// Parse JSON response - array of releases
	var releases []ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse version list: %w", err)
	}

	// Extract versions from tag names
	// Format: RubyInstaller-3.3.10-1
	versionMap := make(map[string]bool)
	versionRegex := regexp.MustCompile(`^RubyInstaller-(\d+\.\d+\.\d+)-\d+$`)

	for _, release := range releases {
		if matches := versionRegex.FindStringSubmatch(release.TagName); len(matches) > 1 {
			version := matches[1]
			versionMap[version] = true
		}
	}

	return p.versionsMapToSlice(versionMap), nil
}

// listAvailableUnix fetches available versions from ruby-builder
func (p *Provider) listAvailableUnix() ([]runtime.AvailableVersion, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Get the toolcache release which contains all Ruby builds
	url := "https://api.github.com/repos/ruby/ruby-builder/releases/tags/toolcache"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "dtvem")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch version list: HTTP %d", resp.StatusCode)
	}

	// Parse JSON response
	var release ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse version list: %w", err)
	}

	// Extract unique versions from asset names
	// Format: ruby-3.4.7-ubuntu-22.04-x64.tar.gz
	versionMap := make(map[string]bool)
	versionRegex := regexp.MustCompile(`^ruby-(\d+\.\d+\.\d+)-`)

	for _, asset := range release.Assets {
		if matches := versionRegex.FindStringSubmatch(asset.Name); len(matches) > 1 {
			version := matches[1]
			// Only include Ruby 2.7+ and 3.x versions (older versions may not have prebuilts)
			if strings.HasPrefix(version, "3.") || strings.HasPrefix(version, "2.7") {
				versionMap[version] = true
			}
		}
	}

	return p.versionsMapToSlice(versionMap), nil
}

// versionsMapToSlice converts a version map to a sorted slice of AvailableVersion
func (p *Provider) versionsMapToSlice(versionMap map[string]bool) []runtime.AvailableVersion {
	// Convert map to sorted slice
	var versionStrings []string
	for version := range versionMap {
		versionStrings = append(versionStrings, version)
	}

	// Sort versions in descending order (newest first)
	sort.Slice(versionStrings, func(i, j int) bool {
		return versionStrings[i] > versionStrings[j]
	})

	// Convert to AvailableVersion format
	versions := make([]runtime.AvailableVersion, 0, len(versionStrings))
	for i, v := range versionStrings {
		notes := ""
		if i == 0 {
			notes = "Latest"
		}
		versions = append(versions, runtime.AvailableVersion{
			Version: runtime.NewVersion(v),
			Notes:   notes,
		})
	}

	return versions
}

// ExecutablePath returns the path to the Ruby executable
func (p *Provider) ExecutablePath(version string) (string, error) {
	installPath, err := p.InstallPath(version)
	if err != nil {
		return "", err
	}

	// Determine executable name and path based on platform
	var rubyPath string
	if goruntime.GOOS == constants.OSWindows {
		// Windows has ruby.exe in bin/
		rubyPath = filepath.Join(installPath, "bin", "ruby.exe")
	} else {
		// Unix has ruby in bin/
		rubyPath = filepath.Join(installPath, "bin", "ruby")
	}

	// Verify executable exists
	if _, err := os.Stat(rubyPath); os.IsNotExist(err) {
		return "", fmt.Errorf("ruby executable not found at %s", rubyPath)
	}

	return rubyPath, nil
}

// IsInstalled checks if a version is installed
func (p *Provider) IsInstalled(version string) (bool, error) {
	installPath := config.RuntimeVersionPath("ruby", version)
	_, err := os.Stat(installPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// InstallPath returns the installation directory for a version
func (p *Provider) InstallPath(version string) (string, error) {
	return config.RuntimeVersionPath("ruby", version), nil
}

// GlobalVersion returns the globally configured version
func (p *Provider) GlobalVersion() (string, error) {
	return config.GlobalVersion("ruby")
}

// SetGlobalVersion sets the global default version
func (p *Provider) SetGlobalVersion(version string) error {
	return config.SetGlobalVersion("ruby", version)
}

// LocalVersion returns the locally configured version
func (p *Provider) LocalVersion() (string, error) {
	version, err := config.ResolveVersion("ruby")
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetLocalVersion sets the local version for current directory
func (p *Provider) SetLocalVersion(version string) error {
	return config.SetLocalVersion("ruby", version)
}

// CurrentVersion returns the currently active version
func (p *Provider) CurrentVersion() (string, error) {
	return config.ResolveVersion("ruby")
}

// DetectInstalled scans the system for existing Ruby installations.
// Note: This method is deprecated. Use migration providers instead
// (rbenv, rvm, chruby, system) for detecting existing installations.
func (p *Provider) DetectInstalled() ([]runtime.DetectedVersion, error) {
	// Detection is now handled by migration providers in src/migrations/
	// This method returns empty to avoid duplicate code
	return []runtime.DetectedVersion{}, nil
}

// GlobalPackages detects globally installed gems
func (p *Provider) GlobalPackages(installPath string) ([]string, error) {
	// Find gem executable in the installation
	gemPath := findGemInInstall(installPath)
	if gemPath == "" {
		return nil, fmt.Errorf("gem not found in installation")
	}

	// Run gem list --no-details
	cmd := exec.Command(gemPath, "list", "--no-details")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list gems: %w", err)
	}

	// Parse output - each line is "gemname (version)" or just "gemname"
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	packages := make([]string, 0, len(lines))

	// Skip default/bundled gems
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

// InstallGlobalPackages reinstalls global gems to a specific version
func (p *Provider) InstallGlobalPackages(version string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	// Get executable path for this version
	execPath, err := p.ExecutablePath(version)
	if err != nil {
		return err
	}

	// Find gem in the same installation
	installDir := filepath.Dir(execPath)
	gemPath := findGemInInstall(installDir)
	if gemPath == "" {
		return fmt.Errorf("gem not found in installation")
	}

	// Install all gems at once
	args := append([]string{"install"}, packages...)
	cmd := exec.Command(gemPath, args...)

	// Capture output for errors
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gem install failed: %w\n%s", err, string(output))
	}

	return nil
}

// ManualPackageInstallCommand returns the command for manually installing gems
func (p *Provider) ManualPackageInstallCommand(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	return fmt.Sprintf("gem install %s", strings.Join(packages, " "))
}

// findGemInInstall finds the gem executable in an installation directory
func findGemInInstall(installDir string) string {
	// Common locations to check
	searchPaths := []string{
		installDir,                       // Same directory
		filepath.Join(installDir, "bin"), // Unix/Windows bin/
	}

	// On Windows, try with .cmd or .bat extension
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
		// On Unix, check without extension
		for _, searchPath := range searchPaths {
			execPath := filepath.Join(searchPath, "gem")
			if _, err := os.Stat(execPath); err == nil {
				return execPath
			}
		}
	}

	return ""
}

// ShouldReshimAfter checks if the given command should trigger a reshim.
// Returns true if the command installs or uninstalls gems.
func (p *Provider) ShouldReshimAfter(shimName string, args []string) bool {
	// gem install/uninstall can add/remove executables
	if shimName == "gem" {
		if len(args) == 0 {
			return false
		}
		cmd := args[0]
		return cmd == "install" || cmd == "uninstall"
	}

	// bundle install/update can add/remove executables via binstubs
	if shimName == "bundle" {
		if len(args) == 0 {
			return false
		}
		cmd := args[0]
		return cmd == "install" || cmd == "update"
	}

	return false
}

// init registers the Ruby provider on package load
func init() {
	if err := runtime.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register Ruby provider: %v", err))
	}
}
