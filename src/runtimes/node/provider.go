// Package node implements the Node.js runtime provider for dtvem
package node

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	goruntime "runtime"
	"strings"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/download"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/shim"
	"github.com/dtvem/dtvem/src/internal/ui"
)

// Provider implements the runtime.Provider interface for Node.js
type Provider struct {
	// Configuration and state will go here
}

// NewProvider creates a new Node.js runtime provider
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the runtime name
func (p *Provider) Name() string {
	return "node"
}

// DisplayName returns the human-readable name
func (p *Provider) DisplayName() string {
	return "Node.js"
}

// Shims returns the list of shim executables for Node.js
func (p *Provider) Shims() []string {
	return []string{"node", "npm", "npx"}
}

// Install downloads and installs a specific version
func (p *Provider) Install(version string) error {
	// Ensure dtvem directories exist
	if err := config.EnsureDirectories(); err != nil {
		return fmt.Errorf("failed to create dtvem directories: %w", err)
	}

	// Check if already installed
	if installed, _ := p.IsInstalled(version); installed {
		return fmt.Errorf("Node.js %s is already installed", version)
	}

	ui.Header("Installing Node.js v%s...", version)

	// Get platform-specific download URL
	downloadURL, archiveName, err := p.getDownloadURL(version)
	if err != nil {
		return fmt.Errorf("failed to get download URL: %w", err)
	}

	ui.Progress("Downloading from %s", downloadURL)

	// Create temporary directory for download
	tempDir := filepath.Join(os.TempDir(), fmt.Sprintf("dtvem-node-%s", version))
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Download archive
	archivePath := filepath.Join(tempDir, archiveName)
	if err := download.File(downloadURL, archivePath); err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}

	// Get install path
	installPath := config.RuntimeVersionPath("node", version)

	// Extract archive with spinner
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
		// Strip top-level directory (Node.js archives have node-v18.16.0/ at the top)
		extractErr = download.StripTopLevelDir(extractDir)
	}

	if extractErr != nil {
		spinner.Error("Extraction failed")
		return fmt.Errorf("failed to extract: %w", extractErr)
	}
	spinner.Success("Extraction complete")

	// Move extracted directory to install location
	if err := os.MkdirAll(filepath.Dir(installPath), 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %w", err)
	}

	if err := os.Rename(extractDir, installPath); err != nil {
		return fmt.Errorf("failed to move to install location: %w", err)
	}

	// Create shims with spinner
	shimSpinner := ui.NewSpinner("Creating shims...")
	shimSpinner.Start()
	if err := p.createShims(); err != nil {
		shimSpinner.Error("Failed to create shims")
		return fmt.Errorf("failed to create shims: %w", err)
	}
	shimSpinner.Success("Shims created")

	ui.Success("Node.js v%s installed successfully", version)
	ui.Info("Location: %s", installPath)

	return nil
}

// getDownloadURL returns the download URL and archive name for a given version
func (p *Provider) getDownloadURL(version string) (string, string, error) {
	// Determine platform and architecture
	platform := goruntime.GOOS
	arch := goruntime.GOARCH

	// Map Go arch to Node.js arch naming
	nodeArch := arch
	if arch == constants.ArchAMD64 {
		nodeArch = "x64"
	} else if arch != constants.ArchARM64 {
		// arm64 is already correct, anything else is unsupported
		return "", "", fmt.Errorf("unsupported architecture: %s", arch)
	}

	// Construct download URL based on platform
	var archiveName string
	var downloadURL string

	switch platform {
	case constants.OSWindows:
		archiveName = fmt.Sprintf("node-v%s-win-%s.zip", version, nodeArch)
		downloadURL = fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, archiveName)

	case "darwin":
		archiveName = fmt.Sprintf("node-v%s-darwin-%s.tar.gz", version, nodeArch)
		downloadURL = fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, archiveName)

	case "linux":
		archiveName = fmt.Sprintf("node-v%s-linux-%s.tar.gz", version, nodeArch)
		downloadURL = fmt.Sprintf("https://nodejs.org/dist/v%s/%s", version, archiveName)

	default:
		return "", "", fmt.Errorf("unsupported platform: %s", platform)
	}

	return downloadURL, archiveName, nil
}

// createShims creates shims for Node.js executables
func (p *Provider) createShims() error {
	manager, err := shim.NewManager()
	if err != nil {
		return err
	}

	// Get the list of shims for Node.js
	shimNames := shim.RuntimeShims("node")

	// Create each shim
	return manager.CreateShims(shimNames)
}

// Uninstall removes an installed version
func (p *Provider) Uninstall(version string) error {
	// TODO: Implement Node.js uninstallation
	return fmt.Errorf("not yet implemented")
}

// ListInstalled returns all installed Node.js versions
func (p *Provider) ListInstalled() ([]runtime.InstalledVersion, error) {
	paths := config.DefaultPaths()
	nodeVersionsDir := filepath.Join(paths.Versions, "node")

	// Check if directory exists
	if _, err := os.Stat(nodeVersionsDir); os.IsNotExist(err) {
		return []runtime.InstalledVersion{}, nil
	}

	// Read directory
	entries, err := os.ReadDir(nodeVersionsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read versions directory: %w", err)
	}

	// Build list of installed versions
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

// ListAvailable returns all available Node.js versions
func (p *Provider) ListAvailable() ([]runtime.AvailableVersion, error) {
	// Fetch version index from nodejs.org
	resp, err := http.Get("https://nodejs.org/dist/index.json")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch version list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch version list: HTTP %d", resp.StatusCode)
	}

	// Parse JSON response
	var nodeVersions []struct {
		Version string      `json:"version"`
		Date    string      `json:"date"`
		LTS     interface{} `json:"lts"` // Can be false or a string like "Hydrogen"
	}

	if err := json.NewDecoder(resp.Body).Decode(&nodeVersions); err != nil {
		return nil, fmt.Errorf("failed to parse version list: %w", err)
	}

	// Convert to AvailableVersion format
	versions := make([]runtime.AvailableVersion, 0, len(nodeVersions))
	for _, v := range nodeVersions {
		// Strip 'v' prefix from version
		version := strings.TrimPrefix(v.Version, "v")

		// Add notes for LTS versions
		notes := ""
		if ltsName, ok := v.LTS.(string); ok && ltsName != "" {
			notes = fmt.Sprintf("LTS: %s", ltsName)
		}

		versions = append(versions, runtime.AvailableVersion{
			Version: runtime.NewVersion(version),
			Notes:   notes,
		})
	}

	return versions, nil
}

// ExecutablePath returns the path to the Node.js executable
func (p *Provider) ExecutablePath(version string) (string, error) {
	installPath, err := p.InstallPath(version)
	if err != nil {
		return "", err
	}

	// Determine executable name and path based on platform
	var nodePath string
	if goruntime.GOOS == constants.OSWindows {
		nodePath = filepath.Join(installPath, "node.exe")
	} else {
		nodePath = filepath.Join(installPath, "bin", "node")
	}

	// Verify executable exists
	if _, err := os.Stat(nodePath); os.IsNotExist(err) {
		return "", fmt.Errorf("node executable not found at %s", nodePath)
	}

	return nodePath, nil
}

// IsInstalled checks if a version is installed
func (p *Provider) IsInstalled(version string) (bool, error) {
	installPath := config.RuntimeVersionPath("node", version)
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
	return config.RuntimeVersionPath("node", version), nil
}

// GlobalVersion returns the globally configured version
func (p *Provider) GlobalVersion() (string, error) {
	return config.GlobalVersion("node")
}

// SetGlobalVersion sets the global default version
func (p *Provider) SetGlobalVersion(version string) error {
	return config.SetGlobalVersion("node", version)
}

// GetLocalVersion returns the locally configured version
func (p *Provider) LocalVersion() (string, error) {
	// Try to find local version file
	version, err := config.ResolveVersion("node")
	if err != nil {
		return "", err
	}
	return version, nil
}

// SetLocalVersion sets the local version for current directory
func (p *Provider) SetLocalVersion(version string) error {
	return config.SetLocalVersion("node", version)
}

// GetCurrentVersion returns the currently active version
func (p *Provider) CurrentVersion() (string, error) {
	return config.ResolveVersion("node")
}

// DetectInstalled scans the system for existing Node.js installations
func (p *Provider) DetectInstalled() ([]runtime.DetectedVersion, error) {
	detected := make([]runtime.DetectedVersion, 0)
	seen := make(map[string]bool) // Track unique paths to avoid duplicates

	// 1. Check node in PATH
	if nodePath, err := exec.LookPath("node"); err == nil {
		if version, err := getNodeVersion(nodePath); err == nil {
			if !seen[nodePath] {
				detected = append(detected, runtime.DetectedVersion{
					Version:   version,
					Path:      nodePath,
					Source:    "system",
					Validated: true,
				})
				seen[nodePath] = true
			}
		}
	}

	// 2. Check common installation locations
	locations := getNodeInstallLocations()
	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			if version, err := getNodeVersion(loc); err == nil {
				if !seen[loc] {
					detected = append(detected, runtime.DetectedVersion{
						Version:   version,
						Path:      loc,
						Source:    "system",
						Validated: true,
					})
					seen[loc] = true
				}
			}
		}
	}

	// 3. Check nvm installations
	nvmVersions := findNvmVersions()
	for _, dv := range nvmVersions {
		if !seen[dv.Path] {
			detected = append(detected, dv)
			seen[dv.Path] = true
		}
	}

	// 4. Check fnm installations
	fnmVersions := findFnmVersions()
	for _, dv := range fnmVersions {
		if !seen[dv.Path] {
			detected = append(detected, dv)
			seen[dv.Path] = true
		}
	}

	return detected, nil
}

// getNodeVersion runs 'node --version' on the given path and returns the version
func getNodeVersion(nodePath string) (string, error) {
	cmd := exec.Command(nodePath, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	version := strings.TrimSpace(string(output))
	// Remove 'v' prefix if present (e.g., "v22.0.0" -> "22.0.0")
	version = strings.TrimPrefix(version, "v")

	return version, nil
}

// getNodeInstallLocations returns common Node.js installation paths
func getNodeInstallLocations() []string {
	home, _ := os.UserHomeDir()

	locations := []string{
		// Windows
		`C:\Program Files\nodejs\node.exe`,
		`C:\Program Files (x86)\nodejs\node.exe`,

		// macOS (Homebrew)
		"/usr/local/bin/node",
		"/opt/homebrew/bin/node",

		// Linux
		"/usr/bin/node",
		"/usr/local/bin/node",
	}

	// Add user-specific locations
	if home != "" {
		locations = append(locations,
			filepath.Join(home, ".local", "bin", "node"),
		)
	}

	return locations
}

// findNvmVersions scans nvm directory for installed versions
func findNvmVersions() []runtime.DetectedVersion {
	detected := make([]runtime.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		return detected
	}

	// Check Unix-style nvm directory
	nvmDir := filepath.Join(home, ".nvm", "versions", "node")
	if entries, err := os.ReadDir(nvmDir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() {
				versionDir := filepath.Join(nvmDir, entry.Name())
				nodePath := filepath.Join(versionDir, "bin", "node")

				if _, err := os.Stat(nodePath); err == nil {
					// Extract version from directory name (e.g., "v22.0.0" -> "22.0.0")
					version := strings.TrimPrefix(entry.Name(), "v")

					detected = append(detected, runtime.DetectedVersion{
						Version:   version,
						Path:      nodePath,
						Source:    "nvm",
						Validated: false,
					})
				}
			}
		}
	}

	// Check Windows nvm directory
	nvmWinDir := filepath.Join(home, "AppData", "Roaming", "nvm")
	if entries, err := os.ReadDir(nvmWinDir); err == nil {
		versionRegex := regexp.MustCompile(`^v?\d+\.\d+\.\d+$`)
		for _, entry := range entries {
			if entry.IsDir() && versionRegex.MatchString(entry.Name()) {
				versionDir := filepath.Join(nvmWinDir, entry.Name())
				nodePath := filepath.Join(versionDir, "node.exe")

				if _, err := os.Stat(nodePath); err == nil {
					version := strings.TrimPrefix(entry.Name(), "v")

					detected = append(detected, runtime.DetectedVersion{
						Version:   version,
						Path:      nodePath,
						Source:    "nvm",
						Validated: false,
					})
				}
			}
		}
	}

	return detected
}

// findFnmVersions scans fnm directory for installed versions
func findFnmVersions() []runtime.DetectedVersion {
	detected := make([]runtime.DetectedVersion, 0)
	home, err := os.UserHomeDir()
	if err != nil {
		return detected
	}

	// fnm stores versions in ~/.local/share/fnm/node-versions or similar
	fnmDirs := []string{
		filepath.Join(home, ".local", "share", "fnm", "node-versions"),
		filepath.Join(home, ".fnm", "node-versions"),
		filepath.Join(home, "Library", "Application Support", "fnm", "node-versions"), // macOS
	}

	for _, fnmDir := range fnmDirs {
		if entries, err := os.ReadDir(fnmDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					versionDir := filepath.Join(fnmDir, entry.Name())

					// Try both Unix and Windows paths
					nodePaths := []string{
						filepath.Join(versionDir, "bin", "node"),
						filepath.Join(versionDir, "node.exe"),
					}

					for _, nodePath := range nodePaths {
						if _, err := os.Stat(nodePath); err == nil {
							version := strings.TrimPrefix(entry.Name(), "v")

							detected = append(detected, runtime.DetectedVersion{
								Version:   version,
								Path:      nodePath,
								Source:    "fnm",
								Validated: false,
							})
							break
						}
					}
				}
			}
		}
	}

	return detected
}

// GetGlobalPackages detects globally installed npm packages
func (p *Provider) GlobalPackages(installPath string) ([]string, error) {
	// Find npm executable in the installation
	npmPath := findNpmInInstall(installPath)
	if npmPath == "" {
		return nil, fmt.Errorf("npm not found in installation")
	}

	// Run npm list -g --depth=0 --json
	cmd := exec.Command(npmPath, "list", "-g", "--depth=0", "--json")
	output, err := cmd.Output()
	if err != nil {
		// npm list returns exit code 1 if there are issues, but might still have output
		// Try to parse anyway
		if len(output) == 0 {
			return []string{}, nil
		}
	}

	// Parse JSON output
	var result struct {
		Dependencies map[string]interface{} `json:"dependencies"`
	}

	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse npm list output: %w", err)
	}

	// Extract package names (exclude npm itself)
	packages := make([]string, 0)
	for name := range result.Dependencies {
		if name != "npm" {
			packages = append(packages, name)
		}
	}

	return packages, nil
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

	// Find npm in the same installation
	installDir := filepath.Dir(execPath)
	npmPath := findNpmInInstall(installDir)
	if npmPath == "" {
		return fmt.Errorf("npm not found in installation")
	}

	// Install all packages at once
	args := append([]string{"install", "-g"}, packages...)
	cmd := exec.Command(npmPath, args...)

	// Capture output for errors
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("npm install failed: %w\n%s", err, string(output))
	}

	return nil
}

// GetManualPackageInstallCommand returns the command for manually installing packages
func (p *Provider) ManualPackageInstallCommand(packages []string) string {
	if len(packages) == 0 {
		return ""
	}
	return fmt.Sprintf("npm install -g %s", strings.Join(packages, " "))
}

// findNpmInInstall finds the npm executable in an installation directory
func findNpmInInstall(installDir string) string {
	// Common locations to check
	searchPaths := []string{
		installDir,                       // Same directory
		filepath.Join(installDir, "bin"), // Unix bin/
	}

	// On Windows, try with .cmd extension (npm uses .cmd on Windows)
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
		// On Unix, check without extension
		for _, searchPath := range searchPaths {
			execPath := filepath.Join(searchPath, "npm")
			if _, err := os.Stat(execPath); err == nil {
				return execPath
			}
		}
	}

	return ""
}

// ShouldReshimAfter checks if the given command should trigger a reshim.
// Returns true if the command installs or uninstalls global packages.
func (p *Provider) ShouldReshimAfter(shimName string, args []string) bool {
	// Only npm installs global packages
	if shimName != "npm" {
		return false
	}

	// Need at least one argument (the command)
	if len(args) == 0 {
		return false
	}

	// Check if this is an install or uninstall command
	cmd := args[0]
	isPackageCommand := cmd == "install" || cmd == "i" ||
		cmd == "uninstall" || cmd == "remove" || cmd == "rm" || cmd == "un"

	if !isPackageCommand {
		return false
	}

	// Check for -g or --global flag
	for _, arg := range args {
		if arg == "-g" || arg == "--global" {
			return true
		}
	}

	return false
}

// init registers the Node.js provider on package load
func init() {
	if err := runtime.Register(NewProvider()); err != nil {
		panic(fmt.Sprintf("failed to register Node.js provider: %v", err))
	}
}
