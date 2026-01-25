// Package python implements the Python runtime provider for dtvem
package python

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/download"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/manifest"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
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
	// python-build-standalone: files are in python/ subdirectory (all platforms)
	pythonSubdir := filepath.Join(extractDir, "python")
	if _, err := os.Stat(pythonSubdir); err == nil {
		return pythonSubdir
	}

	// Fallback: use extractDir if python/ doesn't exist
	// (e.g., Windows embeddable packages from python.org have files in root)
	return extractDir
}

// installPipIfNeeded ensures pip is properly installed and accessible.
// On Windows, pip may be missing (python.org embeddable) or have broken
// executables (python-build-standalone with embedded build paths).
// Running ensurepip --default-pip --upgrade creates working pip executables.
func (p *Provider) installPipIfNeeded(version string) {
	if goruntime.GOOS == constants.OSWindows {
		pipSpinner := ui.NewSpinner("Configuring pip...")
		pipSpinner.Start()
		if err := p.installPip(version); err != nil {
			pipSpinner.Warning("Failed to configure pip")
			ui.Info("To install pip manually, run:")
			ui.Info("  python -m ensurepip --default-pip --upgrade")
		} else {
			pipSpinner.Success("pip configured successfully")
		}
	} else {
		// python-build-standalone includes pip on Unix
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

	// Install/configure pip first (so executables exist before creating shims)
	p.installPipIfNeeded(version)

	// Create shims (after pip is installed, all executables now exist)
	shimSpinner := ui.NewSpinner("Creating shims...")
	shimSpinner.Start()
	if err := p.createShims(); err != nil {
		shimSpinner.Error("Failed to create shims")
		return fmt.Errorf("failed to create shims: %w", err)
	}
	shimSpinner.Success("Shims created")

	ui.Success("Python v%s installed successfully", version)
	ui.Info("Location: %s", installPath)

	return nil
}

// getDownloadURL returns the download URL and archive name for a given version
func (p *Provider) getDownloadURL(version string) (string, string, error) {
	// Get the manifest (uses cached remote with embedded fallback)
	m, err := manifest.DefaultSource().GetManifest("python")
	if err != nil {
		return "", "", fmt.Errorf("failed to load manifest: %w", err)
	}

	// Get the download info for this version and platform
	platform := manifest.CurrentPlatform()
	dl := m.GetDownload(version, platform)
	if dl == nil {
		return "", "", fmt.Errorf("Python %s is not available for %s", version, platform)
	}

	// Extract archive name from URL
	archiveName := filepath.Base(dl.URL)

	return dl.URL, archiveName, nil
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

// installPip ensures pip is properly installed with working executables.
// This handles two scenarios:
// 1. python.org embeddable packages: pip is not included, needs ensurepip
// 2. python-build-standalone: pip module exists but pip.exe has broken paths
//
// Running "python -m ensurepip --default-pip --upgrade" handles both cases
// by (re)installing pip and creating working pip/pip3/pipX.Y executables.
func (p *Provider) installPip(version string) error {
	pythonPath, err := p.ExecutablePath(version)
	if err != nil {
		return fmt.Errorf("could not find python executable: %w", err)
	}

	installPath := config.RuntimeVersionPath("python", version)

	// For python.org embeddable packages, enable site-packages first.
	// This file doesn't exist in python-build-standalone, so errors are ignored.
	pthFile := filepath.Join(installPath, fmt.Sprintf("python%s._pth", strings.Join(strings.Split(version, ".")[:2], "")))
	_ = p.enableSitePackages(pthFile) // Best effort - ignore errors

	// Run ensurepip to install/reinstall pip with working executables.
	// --default-pip: creates pip.exe in addition to pipX.exe and pipX.Y.exe
	// --upgrade: reinstalls even if pip module already exists (fixes broken executables)
	cmd := exec.Command(pythonPath, "-m", "ensurepip", "--default-pip", "--upgrade")
	cmd.Dir = installPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		ui.Debug("ensurepip failed: %v\nOutput: %s", err, string(output))
		// Fall back to get-pip.py for older Python versions or edge cases
		return p.installPipWithGetPip(version, pythonPath, installPath)
	}

	return nil
}

// installPipWithGetPip is a fallback method that downloads and runs get-pip.py.
// Used when ensurepip fails (e.g., ensurepip module missing or corrupted).
func (p *Provider) installPipWithGetPip(version, pythonPath, installPath string) error {
	ui.Debug("Falling back to get-pip.py")

	getPipURL := p.getPipURL(version)
	getPipPath := filepath.Join(installPath, "get-pip.py")
	if err := download.File(getPipURL, getPipPath); err != nil {
		return fmt.Errorf("failed to download get-pip.py: %w", err)
	}
	defer func() { _ = os.Remove(getPipPath) }()

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

// ListAvailable returns all available Python versions
func (p *Provider) ListAvailable() ([]runtime.AvailableVersion, error) {
	// Get the manifest (uses cached remote with embedded fallback)
	m, err := manifest.DefaultSource().GetManifest("python")
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	// Get versions available for current platform
	platform := manifest.CurrentPlatform()
	versionStrings := m.ListAvailableVersions(platform)

	// Convert to AvailableVersion format and sort by semantic version (newest first)
	versions := make([]runtime.AvailableVersion, 0, len(versionStrings))
	for _, v := range versionStrings {
		versions = append(versions, runtime.AvailableVersion{
			Version: runtime.NewVersion(v),
			Notes:   "",
		})
	}

	// Sort by version descending (newest first)
	runtime.SortVersionsDesc(versions)

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
		// Windows: python.exe is in the installation root
		pythonPath = filepath.Join(installPath, "python.exe")
	} else {
		// Unix: python is in bin/ subdirectory
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
