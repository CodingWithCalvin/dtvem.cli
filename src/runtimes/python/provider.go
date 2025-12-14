// Package python implements the Python runtime provider for dtvem
package python

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/download"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/shim"
	"github.com/dtvem/dtvem/src/internal/ui"
)

// Provider implements the runtime.Provider interface for Python
type Provider struct {
	// Configuration and state will go here
}

// NewProvider creates a new Python runtime provider
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the runtime name
func (p *Provider) Name() string {
	return "python"
}

// DisplayName returns the human-readable name
func (p *Provider) DisplayName() string {
	return "Python"
}

// Shims returns the list of shim executables for Python
func (p *Provider) Shims() []string {
	return []string{"python", "python3", "pip", "pip3"}
}

// Install downloads and installs a specific version
// downloadAndExtract downloads and extracts the Python archive
func (p *Provider) downloadAndExtract(version, downloadURL, archiveName string) (extractDir string, cleanup func(), err error) {
	ui.Progress("Downloading from %s", downloadURL)

	// Create temporary directory
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("dtvem-python-%s", version))
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

	// Extract archive
	extractDir = filepath.Join(tempDir, "extracted")
	spinner := ui.NewSpinner("Extracting archive...")
	spinner.Start()

	var extractErr error
	if strings.HasSuffix(archiveName, ".zip") {
		extractErr = download.ExtractZip(archivePath, extractDir)
	} else if strings.HasSuffix(archiveName, ".tar.gz") {
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

// determineSourceDir determines the source directory from extracted archive
func determineSourceDir(extractDir string) string {
	if goruntime.GOOS == constants.OSWindows {
		// Windows embeddable: files are in extractDir root
		return extractDir
	}

	// Unix python-build-standalone: files are in python/ subdirectory
	pythonSubdir := filepath.Join(extractDir, "python")
	if _, err := os.Stat(pythonSubdir); err == nil {
		return pythonSubdir
	}

	// Fallback: use extractDir if python/ doesn't exist
	return extractDir
}

// installPipIfNeeded installs pip on Windows or shows success message on Unix
func (p *Provider) installPipIfNeeded(version string) {
	if goruntime.GOOS == constants.OSWindows {
		// Windows embeddable packages need pip installed
		pipSpinner := ui.NewSpinner("Installing pip...")
		pipSpinner.Start()
		if err := p.installPip(version); err != nil {
			pipSpinner.Warning("Failed to install pip")
			ui.Info("To install pip manually:")
			ui.Info("  1. Download: %s", p.getPipURL(version))
			ui.Info("  2. Run: python get-pip.py")
		} else {
			pipSpinner.Success("pip installed successfully")
		}
	} else {
		// python-build-standalone includes pip
		ui.Success("pip included")
	}
}

func (p *Provider) Install(version string) error {
	ui.Debug("Starting Python installation for version %s", version)

	// Ensure dtvem directories exist
	if err := config.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create dtvem directories: %w", err)
	}

	// Check if already installed
	if installed, _ := p.IsInstalled(version); installed {
		return fmt.Errorf("Python %s is already installed", version)
	}

	ui.Header("Installing Python v%s...", version)

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
	sourceDir := determineSourceDir(extractDir)
	ui.Debug("Source directory: %s", sourceDir)

	// Get install path and move files
	installPath := config.RuntimeVersionPath("python", version)
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

	ui.Success("Python v%s installed successfully", version)
	ui.Info("Location: %s", installPath)

	// Install/verify pip
	p.installPipIfNeeded(version)

	return nil
}

// getDownloadURL returns the download URL and archive name for a given version
func (p *Provider) getDownloadURL(version string) (string, string, error) {
	// Determine platform and architecture
	platform := goruntime.GOOS
	arch := goruntime.GOARCH

	// Construct download URL based on platform
	var archiveName string
	var downloadURL string

	switch platform {
	case constants.OSWindows:
		// Use embeddable package for Windows from python.org
		// Format: python-3.11.0-embed-amd64.zip
		var pythonArch string
		if arch == constants.ArchAMD64 {
			pythonArch = constants.ArchAMD64
		} else if arch == constants.ArchARM64 {
			pythonArch = constants.ArchARM64
		} else {
			return "", "", fmt.Errorf("unsupported Windows architecture: %s", arch)
		}
		archiveName = fmt.Sprintf("python-%s-embed-%s.zip", version, pythonArch)
		downloadURL = fmt.Sprintf("https://www.python.org/ftp/python/%s/%s", version, archiveName)

	case "darwin", "linux":
		// Use python-build-standalone for Unix platforms
		// These are prebuilt Python binaries used by pyenv and other tools
		return p.getStandaloneBuildURL(version, platform, arch)

	default:
		return "", "", fmt.Errorf("unsupported platform: %s", platform)
	}

	return downloadURL, archiveName, nil
}

// getStandaloneBuildURL constructs URL for python-build-standalone releases
func (p *Provider) getStandaloneBuildURL(version, platform, arch string) (string, string, error) {
	// Map Go platform/arch to python-build-standalone naming
	var pbsPlatform string
	var pbsArch string

	// Map architecture
	switch arch {
	case "amd64":
		pbsArch = "x86_64"
	case "arm64":
		pbsArch = "aarch64"
	default:
		return "", "", fmt.Errorf("unsupported architecture for %s: %s", platform, arch)
	}

	// Map platform
	switch platform {
	case "darwin":
		pbsPlatform = "apple-darwin"
	case "linux":
		pbsPlatform = "unknown-linux-gnu"
	default:
		return "", "", fmt.Errorf("unsupported platform: %s", platform)
	}

	// python-build-standalone uses a build date in the version
	// The project moved from indygreg to astral-sh
	// Using release 20240814 which has versions also available on python.org (Windows)
	buildDate := "20240814"

	// Construct archive name
	// Format: cpython-3.11.9+20240814-x86_64-unknown-linux-gnu-install_only.tar.gz
	archiveName := fmt.Sprintf("cpython-%s+%s-%s-%s-install_only.tar.gz",
		version, buildDate, pbsArch, pbsPlatform)

	// Construct download URL
	// https://github.com/astral-sh/python-build-standalone/releases/download/20240814/cpython-...tar.gz
	downloadURL := fmt.Sprintf("https://github.com/astral-sh/python-build-standalone/releases/download/%s/%s",
		buildDate, archiveName)

	return downloadURL, archiveName, nil
}

// createShims creates shims for Python executables
func (p *Provider) createShims() error {
	manager, err := shim.NewManager()
	if err != nil {
		return err
	}

	// Get the list of shims for Python
	shimNames := shim.RuntimeShims("python")

	// Create each shim
	return manager.CreateShims(shimNames)
}

// installPip installs pip for Windows embeddable Python packages
func (p *Provider) installPip(version string) error {
	pythonPath, err := p.ExecutablePath(version)
	if err != nil {
		return fmt.Errorf("could not find python executable: %w", err)
	}

	installPath := config.RuntimeVersionPath("python", version)

	// For embeddable packages, we need to:
	// 1. Modify python311._pth to enable site-packages
	// 2. Download and run get-pip.py

	// Step 1: Enable site-packages by uncommenting the import site line
	pthFile := filepath.Join(installPath, fmt.Sprintf("python%s._pth", strings.Join(strings.Split(version, ".")[:2], "")))
	if err := p.enableSitePackages(pthFile); err != nil {
		return fmt.Errorf("failed to enable site-packages: %w", err)
	}

	// Step 2: Download get-pip.py (use version-specific URL for older Python)
	getPipURL := p.getPipURL(version)
	getPipPath := filepath.Join(installPath, "get-pip.py")
	if err := download.File(getPipURL, getPipPath); err != nil {
		return fmt.Errorf("failed to download get-pip.py: %w", err)
	}
	defer func() { _ = os.Remove(getPipPath) }()

	// Step 3: Run get-pip.py
	cmd := exec.Command(pythonPath, getPipPath)
	cmd.Dir = installPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run get-pip.py: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// getPipURL returns the appropriate get-pip.py URL for the given Python version.
// Older Python versions (3.8 and below) require version-specific URLs since the
// main get-pip.py no longer supports end-of-life Python versions.
func (p *Provider) getPipURL(version string) string {
	parts := strings.Split(version, ".")
	if len(parts) >= 2 && parts[0] == "3" {
		minor, err := strconv.Atoi(parts[1])
		if err == nil && minor <= 8 {
			// Use version-specific URL for Python 3.8 and below
			return fmt.Sprintf("https://bootstrap.pypa.io/pip/%s.%s/get-pip.py", parts[0], parts[1])
		}
	}
	// Default URL for Python 3.9+
	return "https://bootstrap.pypa.io/get-pip.py"
}

// enableSitePackages modifies the ._pth file to enable site-packages
func (p *Provider) enableSitePackages(pthFile string) error {
	// Read the file
	content, err := os.ReadFile(pthFile)
	if err != nil {
		return err
	}

	// Uncomment "import site" line or add it if missing
	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, "import site") {
			// Uncomment if commented
			lines[i] = "import site"
			found = true
			break
		}
	}

	// If not found, add it
	if !found {
		lines = append(lines, "import site")
	}

	// Write back
	newContent := strings.Join(lines, "\n")
	return os.WriteFile(pthFile, []byte(newContent), 0644)
}

// Uninstall removes an installed version
func (p *Provider) Uninstall(version string) error {
	// TODO: Implement Python uninstallation
	return fmt.Errorf("not yet implemented")
}

// ListInstalled returns all installed Python versions
func (p *Provider) ListInstalled() ([]runtime.InstalledVersion, error) {
	paths := config.DefaultPaths()
	pythonVersionsDir := filepath.Join(paths.Versions, "python")

	// Check if directory exists
	if _, err := os.Stat(pythonVersionsDir); os.IsNotExist(err) {
		return []runtime.InstalledVersion{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(pythonVersionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	// Build list of installed versions
	versions := make([]runtime.InstalledVersion, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, runtime.InstalledVersion{
				Version:     runtime.NewVersion(entry.Name()),
				InstallPath: filepath.Join(pythonVersionsDir, entry.Name()),
				IsGlobal:    false, // TODO: Check if this is the global version
			})
		}
	}

	return versions, nil
}

// ListAvailable returns all available Python versions from python.org
func (p *Provider) ListAvailable() ([]runtime.AvailableVersion, error) {
	// Fetch directory listing from python.org FTP
	url := "https://www.python.org/ftp/python/"

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch version list: HTTP %d", resp.StatusCode)
	}

	// Read the HTML body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Extract version numbers from directory listing
	// Directory names look like: 3.11.0/, 3.12.0/, etc.
	versionRegex := regexp.MustCompile(`>(\d+\.\d+\.\d+)/<`)
	matches := versionRegex.FindAllStringSubmatch(string(body), -1)

	versionMap := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			version := match[1]
			// Only include Python 3.x versions
			if strings.HasPrefix(version, "3.") {
				versionMap[version] = true
			}
		}
	}

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

	return versions, nil
}

// ExecutablePath returns the path to the Python executable
func (p *Provider) ExecutablePath(version string) (string, error) {
	installPath, err := p.InstallPath(version)
	if err != nil {
		return "", err
	}

	// Determine executable name and path based on platform
	var pythonPath string
	if goruntime.GOOS == constants.OSWindows {
		// Windows embeddable package has python.exe in root
		pythonPath = filepath.Join(installPath, "python.exe")
	} else {
		// Unix has python in bin/
		pythonPath = filepath.Join(installPath, "bin", "python")
	}

	// Verify executable exists
	if _, err := os.Stat(pythonPath); os.IsNotExist(err) {
		return "", fmt.Errorf("python executable not found at %s", pythonPath)
	}

	return pythonPath, nil
}

// IsInstalled checks if a version is installed
func (p *Provider) IsInstalled(version string) (bool, error) {
	installPath := config.RuntimeVersionPath("python", version)
	_, err := os.Stat(installPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// GetInstallPath returns the installation directory for a version
func (p *Provider) InstallPath(version string) (string, error) {
	return config.RuntimeVersionPath("python", version), nil
}

// GlobalVersion returns the globally configured version
func (p *Provider) GlobalVersion() (string, error) {
	return config.GlobalVersion("python")
}

// SetGlobalVersion sets the global default version
func (p *Provider) SetGlobalVersion(version string) error {
	return config.SetGlobalVersion("python", version)
}

// GetLocalVersion returns the locally configured version
func (p *Provider) LocalVersion() (string, error) {
	// Try to find local version file
	version, err := config.ResolveVersion("python")
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetLocalVersion sets the local version for current directory
func (p *Provider) SetLocalVersion(version string) error {
	return config.SetLocalVersion("python", version)
}

// GetCurrentVersion returns the currently active version
func (p *Provider) CurrentVersion() (string, error) {
	return config.ResolveVersion("python")
}

// DetectInstalled scans the system for existing Python installations.
// Note: This method is deprecated. Use migration providers instead
// (pyenv, system) for detecting existing installations.
func (p *Provider) DetectInstalled() ([]runtime.DetectedVersion, error) {
	// Detection is now handled by migration providers in src/migrations/
	// This method returns empty to avoid duplicate code
	return []runtime.DetectedVersion{}, nil
}

// GetGlobalPackages detects globally installed pip packages
func (p *Provider) GlobalPackages(installPath string) ([]string, error) {
	// Find pip executable in the installation
	pipPath := findPipInInstall(installPath)
	if pipPath == "" {
		return nil, fmt.Errorf("pip not found in installation")
	}

	// Run pip list --format=json
	cmd := exec.Command(pipPath, "list", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list pip packages: %w", err)
	}

	// Parse JSON output
	var packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if err := json.Unmarshal(output, &packages); err != nil {
		return nil, fmt.Errorf("failed to parse pip list output: %w", err)
	}

	// Extract package names (exclude pip and setuptools which are built-in)
	packageNames := make([]string, 0, len(packages))
	for _, pkg := range packages {
		name := strings.ToLower(pkg.Name)
		if name != "pip" && name != "setuptools" && name != "wheel" {
			packageNames = append(packageNames, pkg.Name)
		}
	}

	return packageNames, nil
}

// InstallGlobalPackages reinstalls global packages to a specific version
func (p *Provider) InstallGlobalPackages(version string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	// Get executable path for this version
	execPath, err := p.ExecutablePath(version)
	if err != nil {
		return err
	}

	// Find pip in the same installation
	installDir := filepath.Dir(execPath)
	pipPath := findPipInInstall(installDir)
	if pipPath == "" {
		return fmt.Errorf("pip not found in installation")
	}

	// Install all packages at once
	args := append([]string{"install"}, packages...)
	cmd := exec.Command(pipPath, args...)

	// Capture output for errors
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pip install failed: %w\n%s", err, string(output))
	}

	return nil
}

// GetManualPackageInstallCommand returns the command for manually installing packages
func (p *Provider) ManualPackageInstallCommand(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	return fmt.Sprintf("pip install %s", strings.Join(packages, " "))
}

// findPipInInstall finds the pip executable in an installation directory
func findPipInInstall(installDir string) string {
	// Common locations to check
	searchPaths := []string{
		installDir,                                 // Same directory
		filepath.Join(installDir, "bin"),           // Unix bin/
		filepath.Join(installDir, "Scripts"),       // Python Scripts/ (Windows)
		filepath.Join(installDir, "..", "Scripts"), // Alternative Scripts location
	}

	// On Windows, try with .exe extension
	if goruntime.GOOS == constants.OSWindows {
		for _, searchPath := range searchPaths {
			exePath := filepath.Join(searchPath, "pip.exe")
			if _, err := os.Stat(exePath); err == nil {
				return exePath
			}
		}
	} else {
		// On Unix, check without extension
		for _, searchPath := range searchPaths {
			execPath := filepath.Join(searchPath, "pip")
			if _, err := os.Stat(execPath); err == nil {
				return execPath
			}
		}
	}

	return ""
}

// ShouldReshimAfter checks if the given command should trigger a reshim.
// Returns true if the command installs or uninstalls packages.
func (p *Provider) ShouldReshimAfter(shimName string, args []string) bool {
	// pip, pip3 can install packages with executables
	if shimName != "pip" && shimName != "pip3" {
		return false
	}

	// Need at least one argument (the command)
	if len(args) == 0 {
		return false
	}

	// Check if this is an install or uninstall command
	cmd := args[0]
	return cmd == "install" || cmd == "uninstall"
}

// GetEnvironment returns environment variables needed to run Python binaries.
// Python binaries from python-build-standalone are relocatable and don't require
// special environment setup.
func (p *Provider) GetEnvironment(_ string) (map[string]string, error) {
	return map[string]string{}, nil
}

// init registers the Python provider on package load
func init() {
	if err := runtime.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register Python provider: %v", err))
	}
}
