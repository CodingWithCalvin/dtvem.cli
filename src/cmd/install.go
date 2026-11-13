package cmd

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/version"
	"github.com/spf13/cobra"
)

var (
	installYesFlag    bool
	installSignedFlag bool
)

var installCmd = &cobra.Command{
	Use:   "install [runtime] [version]",
	Short: "Install runtime version(s)",
	Long: `Install a specific version of a runtime, or install all runtimes from .dtvem/runtimes.json.

Single install (exact version):
  dtvem install python 3.11.0
  dtvem install node 18.16.0

Single install (partial version - resolves to latest match):
  dtvem install node 22       # Installs latest 22.x.x (e.g., 22.15.0)
  dtvem install python 3.12   # Installs latest 3.12.x (e.g., 3.12.1)

Bulk install (reads .dtvem/runtimes.json):
  dtvem install
  dtvem install --yes    # Skip confirmation prompt

Signed Python builds (Windows):
  dtvem install python 3.12.10 --signed  # python.org's own build
  dtvem install python 3.12 --signed     # Newest 3.12.x with a python.org build

  On Windows dtvem installs python-build-standalone's build of Python by
  default, which is not Authenticode-signed. --signed installs python.org's
  build instead, signed by the Python Software Foundation. python.org ships
  no Windows binaries for security-only releases (e.g. 3.12.11 on), so
  those versions have no signed build. 'dtvem list-all python' shows which
  versions do.`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 || len(args) == 2 {
			return nil
		}
		return fmt.Errorf("accepts 0 or 2 arg(s), received %d", len(args))
	},
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 2 {
			// Single install mode
			installSingle(args[0], args[1])
		} else {
			// Bulk install mode
			installBulk()
		}
	},
}

func init() {
	rootCmd.AddCommand(installCmd)
	installCmd.Flags().BoolVarP(&installYesFlag, "yes", "y", false, "Skip confirmation prompt")
	installCmd.Flags().BoolVar(&installSignedFlag, "signed", false, "Install python.org's Authenticode-signed build (Python on Windows)")
}

// installSingle installs a single runtime/version
func installSingle(runtimeName, versionInput string) {
	ui.Debug("Installing single runtime: %s version %s", runtimeName, versionInput)

	provider, err := runtime.Get(runtimeName)
	if err != nil {
		ui.Debug("Provider lookup failed: %v", err)
		ui.Error("%v", err)
		ui.Info("Available runtimes: %v", runtime.List())
		os.Exit(1)
	}

	ui.Debug("Using provider: %s (%s)", provider.Name(), provider.DisplayName())

	var signed runtime.SignedBuildProvider
	if installSignedFlag {
		if signed, err = signedBuildProvider(provider); err != nil {
			ui.Error("%v", err)
			os.Exit(1)
		}
	}

	// Resolve partial version to full version if needed
	var resolvedVersion string
	if signed != nil {
		resolvedVersion, err = resolveSignedVersion(provider, signed, versionInput)
	} else {
		resolvedVersion, err = resolveVersionForProvider(provider, versionInput)
	}
	if err != nil {
		ui.Debug("Version resolution failed: %v", err)
		ui.Error("%v", err)
		os.Exit(1)
	}

	// Inform user if version was resolved from partial input
	if resolvedVersion != versionInput {
		if signed != nil {
			ui.Info("Resolved %s to %s, the newest %s with a signed build", versionInput, resolvedVersion, versionInput)
		} else {
			ui.Info("Resolved %s to %s", versionInput, resolvedVersion)
		}
	}

	if signed != nil {
		err = signed.InstallSigned(resolvedVersion)
	} else {
		err = provider.Install(resolvedVersion)
	}
	if err != nil {
		ui.Debug("Installation failed: %v", err)
		ui.Error("%v", err)
		showNoSignedBuildHelp(provider, err)
		os.Exit(1)
	}

	ui.Success("Successfully installed %s %s", provider.DisplayName(), resolvedVersion)
	if signed != nil {
		warnNewerUnsigned(provider, signed, resolvedVersion)
	}

	// Auto-set global version if no global version is currently configured
	autoSetGlobalIfNeeded(provider, resolvedVersion)
}

// signedBuildProvider returns provider's support for signed builds, or an
// error when it has none on this platform.
func signedBuildProvider(provider runtime.Provider) (runtime.SignedBuildProvider, error) {
	signed, ok := provider.(runtime.SignedBuildProvider)
	if !ok {
		return nil, fmt.Errorf("--signed is only supported for Python, not %s", provider.DisplayName())
	}
	if !signed.SignedBuildsSupported() {
		return nil, errors.New("--signed is only supported on Windows: python.org's signed builds are Windows builds")
	}
	return signed, nil
}

// resolveSignedVersion resolves a partial version to the newest version
// with a signed build. A full version is returned as-is; installing it
// reports when it has no signed build.
func resolveSignedVersion(provider runtime.Provider, signed runtime.SignedBuildProvider, input string) (string, error) {
	if !version.IsPartialVersion(input) {
		return strings.TrimPrefix(input, "v"), nil
	}

	versions, err := signed.SignedVersions()
	if err != nil {
		return "", fmt.Errorf("failed to fetch signed versions: %w", err)
	}
	resolved, err := version.ResolvePartialVersion(input, versions)
	if err != nil {
		return "", fmt.Errorf("no %s version matching %q has a signed build; 'dtvem list-all %s' shows which versions do", provider.DisplayName(), input, provider.Name())
	}
	return resolved, nil
}

// showNoSignedBuildHelp says what to do instead when err is a
// *runtime.NoSignedBuildError.
func showNoSignedBuildHelp(provider runtime.Provider, err error) {
	var noSigned *runtime.NoSignedBuildError
	if !errors.As(err, &noSigned) {
		return
	}
	if noSigned.Nearest != "" {
		ui.Info("Install the signed build of %s with: dtvem install %s %s --signed", noSigned.Nearest, provider.Name(), noSigned.Nearest)
	}
	if !noSigned.NoneListed {
		ui.Info("Or install %s %s without --signed to get its default build, which is not signed", provider.DisplayName(), noSigned.Version)
	}
}

// warnNewerUnsigned warns when newer releases on the major.minor line of a
// signed version have no signed build: they may carry security fixes the
// signed version lacks.
func warnNewerUnsigned(provider runtime.Provider, signed runtime.SignedBuildProvider, installed string) {
	available, err := provider.ListAvailable()
	if err != nil {
		return
	}
	signedVersions, err := signed.SignedVersions()
	if err != nil {
		return
	}
	if newer := newerUnsigned(installed, available, signedVersions); len(newer) > 0 {
		ui.Warning("%s", newerUnsignedWarning(provider.DisplayName(), installed, newer))
	}
}

// newerUnsigned returns the versions in available that are newer than
// installed, on its major.minor line, and not in signedVersions.
func newerUnsigned(installed string, available []runtime.AvailableVersion, signedVersions []string) []string {
	hasSigned := make(map[string]bool, len(signedVersions))
	for _, v := range signedVersions {
		hasSigned[v] = true
	}
	var newer []string
	for _, av := range available {
		v := av.Version.Raw
		if !hasSigned[v] && version.MajorMinor(v) == version.MajorMinor(installed) && version.Compare(v, installed) > 0 {
			newer = append(newer, v)
		}
	}
	return newer
}

// newerUnsignedWarning words the warning that versions newer than installed
// on its line have no signed build.
func newerUnsignedWarning(displayName, installed string, newer []string) string {
	sort.Slice(newer, func(i, j int) bool { return version.Compare(newer[i], newer[j]) > 0 })
	if len(newer) == 1 {
		return fmt.Sprintf("%s %s is newer but has no signed build; it may carry security fixes %s lacks",
			displayName, newer[0], installed)
	}
	return fmt.Sprintf("%s %s through %s are newer but have no signed build; they may carry security fixes %s lacks",
		displayName, newer[len(newer)-1], newer[0], installed)
}

// autoSetGlobalIfNeeded sets the installed version as global if no global version exists
func autoSetGlobalIfNeeded(provider runtime.Provider, ver string) {
	currentGlobal, err := provider.GlobalVersion()
	if err != nil || currentGlobal != "" {
		// Either an error occurred or a global version is already set
		ui.Debug("Global version check: current=%q, err=%v", currentGlobal, err)
		return
	}

	// No global version configured, auto-set it
	if err := provider.SetGlobalVersion(ver); err != nil {
		ui.Debug("Failed to auto-set global version: %v", err)
		ui.Warning("Could not auto-set global version: %v", err)
		return
	}

	ui.Info("Set as global version (first install)")
}

// resolveVersionForProvider resolves a partial version input to a full version.
// If the input is already a full version (3 components), it's returned as-is.
// For partial versions (1-2 components), it finds the highest matching version.
func resolveVersionForProvider(provider runtime.Provider, input string) (string, error) {
	// If it's already a full version, return as-is
	if !version.IsPartialVersion(input) {
		return strings.TrimPrefix(input, "v"), nil
	}

	// Get available versions from the provider
	available, err := provider.ListAvailable()
	if err != nil {
		return "", fmt.Errorf("failed to fetch available versions: %w", err)
	}

	// Extract version strings
	versionStrings := make([]string, len(available))
	for i, av := range available {
		versionStrings[i] = av.Version.Raw
	}

	// Resolve the partial version
	resolved, err := version.ResolvePartialVersion(input, versionStrings)
	if err != nil {
		return "", fmt.Errorf("no %s version matching %q found", provider.DisplayName(), input)
	}

	return resolved, nil
}

// installBulk installs all runtimes from .dtvem/runtimes.json
// installTask represents a runtime version to be installed
type installTask struct {
	runtimeName      string
	version          string
	provider         runtime.Provider
	alreadyInstalled bool
	// signed is set when --signed asks for this runtime's signed build.
	signed runtime.SignedBuildProvider
}

// buildInstallTasks creates a list of install tasks from the config
func buildInstallTasks(runtimes map[string]string) []installTask {
	var tasks []installTask

	ui.Info("Checking which versions need to be installed...")
	for runtimeName, version := range runtimes {
		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Warning("Unknown runtime '%s', skipping", runtimeName)
			continue
		}

		alreadyInstalled := isVersionInstalled(provider, version)

		task := installTask{
			runtimeName:      runtimeName,
			version:          version,
			provider:         provider,
			alreadyInstalled: alreadyInstalled,
		}
		if installSignedFlag {
			// --signed applies to the runtimes that have signed builds;
			// the others install as usual.
			task.signed, _ = provider.(runtime.SignedBuildProvider)
		}
		tasks = append(tasks, task)
	}

	return tasks
}

// checkSignedTasks fails when --signed asks for signed builds this platform
// does not offer, rather than quietly installing unsigned ones.
func checkSignedTasks(tasks []installTask) error {
	for _, task := range tasks {
		if task.signed == nil {
			continue
		}
		if _, err := signedBuildProvider(task.provider); err != nil {
			return err
		}
	}
	return nil
}

// isVersionInstalled checks if a specific version is already installed
func isVersionInstalled(provider runtime.Provider, version string) bool {
	installedVersions, err := provider.ListInstalled()
	if err != nil {
		return false
	}

	for _, installed := range installedVersions {
		if installed.Version.Raw == version {
			return true
		}
	}

	return false
}

// showInstallationPlan displays the installation plan and returns counts
func showInstallationPlan(tasks []installTask) (toInstall, alreadyInstalled int) {
	ui.Header("\nInstallation Plan:")

	for _, task := range tasks {
		switch {
		case task.alreadyInstalled && task.signed != nil && !task.signed.InstalledBuild(task.version).Signed:
			ui.Warning("  ✓ %s %s (already installed, not a signed build; uninstall it to install the signed build)", task.provider.DisplayName(), task.version)
			alreadyInstalled++
		case task.alreadyInstalled:
			ui.Info("  ✓ %s %s (already installed)", task.provider.DisplayName(), task.version)
			alreadyInstalled++
		case task.signed != nil:
			ui.Info("  → %s %s (will install the signed build)", task.provider.DisplayName(), task.version)
			toInstall++
		default:
			ui.Info("  → %s %s (will install)", task.provider.DisplayName(), task.version)
			toInstall++
		}
	}

	return toInstall, alreadyInstalled
}

// promptInstallConfirmation prompts the user to confirm installation
func promptInstallConfirmation(toInstallCount, alreadyInstalledCount int) bool {
	if installYesFlag {
		return true
	}

	ui.Info("\n%d runtime(s) will be installed, %d already installed", toInstallCount, alreadyInstalledCount)
	ui.Info("Continue? [Y/n]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	return response == "" || response == constants.ResponseY || response == constants.ResponseYes
}

// executeInstalls installs all tasks and returns counts and failures
func executeInstalls(tasks []installTask) (success, failures int, failureList []string) {
	ui.Header("\nInstalling runtimes...")

	for _, task := range tasks {
		if task.alreadyInstalled {
			continue
		}

		ui.Progress("Installing %s %s...", task.provider.DisplayName(), task.version)

		var err error
		if task.signed != nil {
			err = task.signed.InstallSigned(task.version)
		} else {
			err = task.provider.Install(task.version)
		}
		if err != nil {
			ui.Error("Failed to install %s %s: %v", task.provider.DisplayName(), task.version, err)
			showNoSignedBuildHelp(task.provider, err)
			failures++
			failureList = append(failureList, fmt.Sprintf("%s %s", task.provider.DisplayName(), task.version))
		} else {
			ui.Success("Installed %s %s", task.provider.DisplayName(), task.version)
			if task.signed != nil {
				warnNewerUnsigned(task.provider, task.signed, task.version)
			}
			success++
			// Auto-set global version if needed
			autoSetGlobalIfNeeded(task.provider, task.version)
		}
	}

	return success, failures, failureList
}

// showInstallSummary displays the final installation summary
func showInstallSummary(successCount, alreadyInstalledCount, failureCount int, failures []string) {
	ui.Header("\nInstallation Summary:")

	if successCount > 0 {
		ui.Success("Successfully installed: %d runtime(s)", successCount)
	}
	if alreadyInstalledCount > 0 {
		ui.Info("Already installed: %d runtime(s)", alreadyInstalledCount)
	}
	if failureCount > 0 {
		ui.Error("Failed to install: %d runtime(s)", failureCount)
		for _, failure := range failures {
			ui.Error("  - %s", failure)
		}
	}

	if failureCount == 0 {
		ui.Success("\n✓ All runtimes installed successfully!")
	}
}

func installBulk() {
	ui.Header("Bulk Install from runtimes.json")

	// Find and read config file
	configPath, err := config.FindLocalRuntimesFile()
	if err != nil {
		ui.Error("No .dtvem/runtimes.json file found in current directory or parent directories")
		ui.Info("Create one with: dtvem freeze")
		ui.Info("Or manually create .dtvem/runtimes.json with content like:")
		ui.Info(`  {
    "python": "3.11.0",
    "node": "18.16.0"
  }`)
		os.Exit(1)
	}

	ui.Info("Found config: %s", configPath)

	runtimes, err := config.ReadAllRuntimes(configPath)
	if err != nil {
		ui.Error("Failed to read config file: %v", err)
		os.Exit(1)
	}

	if len(runtimes) == 0 {
		ui.Warning("No runtimes found in config file")
		return
	}

	// Build install tasks
	tasks := buildInstallTasks(runtimes)
	if err := checkSignedTasks(tasks); err != nil {
		ui.Error("%v", err)
		os.Exit(1)
	}

	// Show installation plan
	toInstallCount, alreadyInstalledCount := showInstallationPlan(tasks)

	if toInstallCount == 0 {
		ui.Success("\nAll runtimes are already installed!")
		return
	}

	// Prompt for confirmation
	if !promptInstallConfirmation(toInstallCount, alreadyInstalledCount) {
		ui.Info("Installation canceled")
		return
	}

	// Execute installations
	successCount, failureCount, failures := executeInstalls(tasks)

	// Show final summary
	showInstallSummary(successCount, alreadyInstalledCount, failureCount, failures)

	// Exit with error if any installations failed
	if failureCount > 0 {
		os.Exit(1)
	}
}
