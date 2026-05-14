//go:build !shim

// This file holds the "full half" of the Python provider: methods that
// install, list, migrate, and otherwise touch the network or extract archives.
// Excluded from shim builds so the shim binary doesn't link net/http, sevenzip,
// klauspost/compress, brotli, lz4, xz, embedded manifests, etc.
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

// downloadAndExtract downloads and extracts the Python archive.
func (p *Provider) downloadAndExtract(version, downloadURL, archiveName string) (extractDir string, cleanup func(), err error) {
	ui.Progress("Downloading from %s", downloadURL)

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("dtvem-python-%s", version))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return "", nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	cleanupFunc := func() { _ = os.RemoveAll(tempDir) }

	archivePath := filepath.Join(tempDir, archiveName)
	if err := download.File(downloadURL, archivePath); err != nil {
		cleanupFunc()
		return "", nil, fmt.Errorf("failed to download: %w", err)
	}

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

// determineSourceDir determines the source directory from extracted archive.
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

// Install downloads and installs a specific version.
func (p *Provider) Install(version string) error {
	ui.Debug("Starting Python installation for version %s", version)

	if err := config.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create dtvem directories: %w", err)
	}

	if installed, _ := p.IsInstalled(version); installed {
		return fmt.Errorf("Python %s is already installed", version)
	}

	ui.Header("Installing Python v%s...", version)

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

	sourceDir := determineSourceDir(extractDir)
	ui.Debug("Source directory: %s", sourceDir)

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

	shimSpinner := ui.NewSpinner("Creating shims...")
	shimSpinner.Start()
	if err := p.createShims(version); err != nil {
		shimSpinner.Error("Failed to create shims")
		return fmt.Errorf("failed to create shims: %w", err)
	}
	shimSpinner.Success("Shims created")

	ui.Success("Python v%s installed successfully", version)
	ui.Info("Location: %s", installPath)

	return nil
}

// getDownloadURL returns the download URL and archive name for a given version.
func (p *Provider) getDownloadURL(version string) (string, string, error) {
	m, err := manifest.DefaultSource().GetManifest("python")
	if err != nil {
		return "", "", fmt.Errorf("failed to load manifest: %w", err)
	}

	platform := manifest.CurrentPlatform()
	dl := m.GetDownload(version, platform)
	if dl == nil {
		return "", "", fmt.Errorf("Python %s is not available for %s", version, platform)
	}

	archiveName := filepath.Base(dl.URL)

	return dl.URL, archiveName, nil
}

// createShims creates shims for Python executables and registers them in the
// shim-map cache so subsequent shim invocations resolve via O(1) lookup rather
// than falling back to the provider registry. The version is recorded in the
// cache so the shim can detect when an active runtime version is one that
// does not provide a given executable.
//
// The shim list is derived from disk (the same scan reshim uses), not from
// the provider's static Shims() declaration. That keeps install and reshim
// honest about which executables actually exist: on Windows the upstream
// python-build-standalone tarball ships only python.exe / pythonw.exe — no
// python3.exe, no pip.exe — so the static list would create phantom shims
// (python3, pip3) that error at invocation time. Pip executables only
// appear in Scripts/ after installPipIfNeeded succeeds, and scanning here
// ensures they're picked up when present and silently skipped when not.
func (p *Provider) createShims(version string) error {
	manager, err := shim.NewManager()
	if err != nil {
		return err
	}

	versionDir := config.RuntimeVersionPath("python", version)
	shimNames := shim.DiscoverShimsForVersion(versionDir)
	if len(shimNames) == 0 {
		return fmt.Errorf("no executables found in %s", versionDir)
	}

	return manager.CreateShimsForRuntime("python", version, shimNames)
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
	cmd := exec.Command(pythonPath, "-m", "ensurepip", "--default-pip", "--upgrade")
	cmd.Dir = installPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		ui.Debug("ensurepip failed: %v\nOutput: %s", err, string(output))
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
			return fmt.Sprintf("https://bootstrap.pypa.io/pip/%s.%s/get-pip.py", parts[0], parts[1])
		}
	}
	return "https://bootstrap.pypa.io/get-pip.py"
}

// enableSitePackages modifies the ._pth file to enable site-packages.
func (p *Provider) enableSitePackages(pthFile string) error {
	content, err := os.ReadFile(pthFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.Contains(line, "import site") {
			lines[i] = "import site"
			found = true
			break
		}
	}

	if !found {
		lines = append(lines, "import site")
	}

	newContent := strings.Join(lines, "\n")
	return os.WriteFile(pthFile, []byte(newContent), 0644)
}

// Uninstall removes an installed version.
func (p *Provider) Uninstall(version string) error {
	// TODO: Implement Python uninstallation
	return fmt.Errorf("not yet implemented")
}

// ListInstalled returns all installed Python versions.
func (p *Provider) ListInstalled() ([]runtime.InstalledVersion, error) {
	paths := config.DefaultPaths()
	pythonVersionsDir := filepath.Join(paths.Versions, "python")

	if _, err := os.Stat(pythonVersionsDir); os.IsNotExist(err) {
		return []runtime.InstalledVersion{}, nil
	}

	entries, err := os.ReadDir(pythonVersionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

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

// ListAvailable returns all available Python versions.
func (p *Provider) ListAvailable() ([]runtime.AvailableVersion, error) {
	m, err := manifest.DefaultSource().GetManifest("python")
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
	return config.GlobalVersion("python")
}

// SetGlobalVersion sets the global default version.
func (p *Provider) SetGlobalVersion(version string) error {
	return config.SetGlobalVersion("python", version)
}

// LocalVersion returns the locally configured version.
func (p *Provider) LocalVersion() (string, error) {
	version, err := config.ResolveVersion("python")
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetLocalVersion sets the local version for current directory.
func (p *Provider) SetLocalVersion(version string) error {
	return config.SetLocalVersion("python", version)
}

// CurrentVersion returns the currently active version.
func (p *Provider) CurrentVersion() (string, error) {
	return config.ResolveVersion("python")
}

// DetectInstalled scans the system for existing Python installations.
// Detection is handled by migration providers in src/migrations/; this
// method returns empty to avoid duplicate code.
func (p *Provider) DetectInstalled() ([]runtime.DetectedVersion, error) {
	return []runtime.DetectedVersion{}, nil
}

// GlobalPackages detects globally installed pip packages.
func (p *Provider) GlobalPackages(installPath string) ([]string, error) {
	pipPath := findPipInInstall(installPath)
	if pipPath == "" {
		return nil, fmt.Errorf("pip not found in installation")
	}

	cmd := exec.Command(pipPath, "list", "--format=json")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list pip packages: %w", err)
	}

	var packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}

	if err := json.Unmarshal(output, &packages); err != nil {
		return nil, fmt.Errorf("failed to parse pip list output: %w", err)
	}

	packageNames := make([]string, 0, len(packages))
	for _, pkg := range packages {
		name := strings.ToLower(pkg.Name)
		if name != "pip" && name != "setuptools" && name != "wheel" {
			packageNames = append(packageNames, pkg.Name)
		}
	}

	return packageNames, nil
}

// InstallGlobalPackages reinstalls global packages to a specific version.
func (p *Provider) InstallGlobalPackages(version string, packages []string) error {
	if len(packages) == 0 {
		return nil
	}

	execPath, err := p.ExecutablePath(version)
	if err != nil {
		return err
	}

	installDir := filepath.Dir(execPath)
	pipPath := findPipInInstall(installDir)
	if pipPath == "" {
		return fmt.Errorf("pip not found in installation")
	}

	args := append([]string{"install"}, packages...)
	cmd := exec.Command(pipPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pip install failed: %w\n%s", err, string(output))
	}

	return nil
}

// ManualPackageInstallCommand returns the command for manually installing packages.
func (p *Provider) ManualPackageInstallCommand(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	return fmt.Sprintf("pip install %s", strings.Join(packages, " "))
}

// findPipInInstall finds the pip executable in an installation directory.
func findPipInInstall(installDir string) string {
	searchPaths := []string{
		installDir,
		filepath.Join(installDir, "bin"),
		filepath.Join(installDir, "Scripts"),
		filepath.Join(installDir, "..", "Scripts"),
	}

	if goruntime.GOOS == constants.OSWindows {
		for _, searchPath := range searchPaths {
			exePath := filepath.Join(searchPath, "pip.exe")
			if _, err := os.Stat(exePath); err == nil {
				return exePath
			}
		}
	} else {
		for _, searchPath := range searchPaths {
			execPath := filepath.Join(searchPath, "pip")
			if _, err := os.Stat(execPath); err == nil {
				return execPath
			}
		}
	}

	return ""
}
