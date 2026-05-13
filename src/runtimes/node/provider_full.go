//go:build !shim

// This file holds the "full half" of the Node.js provider: methods that
// install, list, migrate, and otherwise touch the network or extract archives.
// Excluded from shim builds so the shim binary doesn't link net/http, sevenzip,
// klauspost/compress, brotli, lz4, xz, embedded manifests, etc.
package node

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	if err := config.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create dtvem directories: %w", err)
	}

	if installed, _ := p.IsInstalled(version); installed {
		return fmt.Errorf("Node.js %s is already installed", version)
	}

	ui.Header("Installing Node.js v%s...", version)

	downloadURL, archiveName, err := p.getDownloadURL(version)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	ui.Progress("Downloading from %s", downloadURL)

	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("dtvem-node-%s", version))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	archivePath := filepath.Join(tempDir, archiveName)
	if err := download.File(downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	installPath := config.RuntimeVersionPath("node", version)

	extractDir := filepath.Join(tempDir, "extracted")
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

	if extractErr == nil {
		extractErr = download.StripTopLevelDir(extractDir)
	}

	if extractErr != nil {
		spinner.Error("Extraction failed")
		return fmt.Errorf("failed to extract: %w", extractErr)
	}
	spinner.Success("Extraction complete")

	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	if err := os.Rename(extractDir, installPath); err != nil {
		return fmt.Errorf("failed to move to install location: %w", err)
	}

	shimSpinner := ui.NewSpinner("Creating shims...")
	shimSpinner.Start()
	if err := p.createShims(version); err != nil {
		shimSpinner.Error("Failed to create shims")
		return fmt.Errorf("failed to create shims: %w", err)
	}
	shimSpinner.Success("Shims created")

	ui.Success("Node.js v%s installed successfully", version)
	ui.Info("Location: %s", installPath)

	return nil
}

// getDownloadURL returns the download URL and archive name for a given version.
func (p *Provider) getDownloadURL(version string) (string, string, error) {
	m, err := manifest.DefaultSource().GetManifest("node")
	if err != nil {
		return "", "", fmt.Errorf("failed to load manifest: %w", err)
	}

	platform := manifest.CurrentPlatform()
	dl := m.GetDownload(version, platform)
	if dl == nil {
		return "", "", fmt.Errorf("Node.js %s is not available for %s", version, platform)
	}

	archiveName := filepath.Base(dl.URL)

	return dl.URL, archiveName, nil
}

// createShims creates shims for Node.js executables and registers them in the
// shim-map cache so subsequent shim invocations resolve via O(1) lookup rather
// than falling back to the provider registry. The version is recorded in the
// cache so the shim can detect when an active runtime version is one that
// does not provide a given executable.
func (p *Provider) createShims(version string) error {
	manager, err := shim.NewManager()
	if err != nil {
		return err
	}

	shimNames := shim.RuntimeShims("node")

	return manager.CreateShimsForRuntime("node", version, shimNames)
}

// Uninstall removes an installed version.
func (p *Provider) Uninstall(version string) error {
	// TODO: Implement Node.js uninstallation
	return fmt.Errorf("not yet implemented")
}

// ListInstalled returns all installed Node.js versions.
func (p *Provider) ListInstalled() ([]runtime.InstalledVersion, error) {
	paths := config.DefaultPaths()
	nodeVersionsDir := filepath.Join(paths.Versions, "node")

	if _, err := os.Stat(nodeVersionsDir); os.IsNotExist(err) {
		return []runtime.InstalledVersion{}, nil
	}

	entries, err := os.ReadDir(nodeVersionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	versions := make([]runtime.InstalledVersion, 0)
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, runtime.InstalledVersion{
				Version:     runtime.NewVersion(entry.Name()),
				InstallPath: filepath.Join(nodeVersionsDir, entry.Name()),
				IsGlobal:    false, // TODO: Check if this is the global version
			})
		}
	}

	return versions, nil
}

// ListAvailable returns all available Node.js versions.
func (p *Provider) ListAvailable() ([]runtime.AvailableVersion, error) {
	m, err := manifest.DefaultSource().GetManifest("node")
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}

	platform := manifest.CurrentPlatform()
	versionStrings := m.ListAvailableVersions(platform)

	lp := newLifecycleProvider()

	versions := make([]runtime.AvailableVersion, 0, len(versionStrings))
	for _, v := range versionStrings {
		versions = append(versions, runtime.AvailableVersion{
			Version:         runtime.NewVersion(v),
			LifecycleStatus: lp.VersionStatus(v),
		})
	}

	runtime.SortVersionsDesc(versions)

	return versions, nil
}

// GlobalVersion returns the globally configured version.
func (p *Provider) GlobalVersion() (string, error) {
	return config.GlobalVersion("node")
}

// SetGlobalVersion sets the global default version.
func (p *Provider) SetGlobalVersion(version string) error {
	return config.SetGlobalVersion("node", version)
}

// LocalVersion returns the locally configured version.
func (p *Provider) LocalVersion() (string, error) {
	version, err := config.ResolveVersion("node")
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetLocalVersion sets the local version for current directory.
func (p *Provider) SetLocalVersion(version string) error {
	return config.SetLocalVersion("node", version)
}

// CurrentVersion returns the currently active version.
func (p *Provider) CurrentVersion() (string, error) {
	return config.ResolveVersion("node")
}

// DetectInstalled scans the system for existing Node.js installations.
// Detection is handled by migration providers in src/migrations/; this
// method returns empty to avoid duplicate code.
func (p *Provider) DetectInstalled() ([]runtime.DetectedVersion, error) {
	return []runtime.DetectedVersion{}, nil
}

// GlobalPackages detects globally installed npm packages.
func (p *Provider) GlobalPackages(installPath string) ([]string, error) {
	npmPath := findNpmInInstall(installPath)
	if npmPath == "" {
		return nil, fmt.Errorf("npm not found in installation")
	}

	cmd := exec.Command(npmPath, "list", "-g", "--depth=0", "--json")
	output, err := cmd.Output()
	if err != nil {
		// npm list returns exit code 1 if there are issues, but might still have output
		if len(output) == 0 {
			return []string{}, nil
		}
	}

	var result struct {
		Dependencies map[string]interface{} `json:"dependencies"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse npm list output: %w", err)
	}

	packages := make([]string, 0)
	for name := range result.Dependencies {
		if name != "npm" {
			packages = append(packages, name)
		}
	}

	return packages, nil
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
	npmPath := findNpmInInstall(installDir)
	if npmPath == "" {
		return fmt.Errorf("npm not found in installation")
	}

	args := append([]string{"install", "-g"}, packages...)
	cmd := exec.Command(npmPath, args...)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm install failed: %w\n%s", err, string(output))
	}

	return nil
}

// ManualPackageInstallCommand returns the command for manually installing packages.
func (p *Provider) ManualPackageInstallCommand(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	return fmt.Sprintf("npm install -g %s", strings.Join(packages, " "))
}

// findNpmInInstall finds the npm executable in an installation directory.
func findNpmInInstall(installDir string) string {
	searchPaths := []string{
		installDir,
		filepath.Join(installDir, "bin"),
	}

	if goruntime.GOOS == constants.OSWindows {
		for _, searchPath := range searchPaths {
			cmdPath := filepath.Join(searchPath, "npm.cmd")
			if _, err := os.Stat(cmdPath); err == nil {
				return cmdPath
			}
			exePath := filepath.Join(searchPath, "npm.exe")
			if _, err := os.Stat(exePath); err == nil {
				return exePath
			}
		}
	} else {
		for _, searchPath := range searchPaths {
			execPath := filepath.Join(searchPath, "npm")
			if _, err := os.Stat(execPath); err == nil {
				return execPath
			}
		}
	}

	return ""
}
