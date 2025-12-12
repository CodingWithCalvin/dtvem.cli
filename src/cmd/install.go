package cmd

import (
	"fmt"
	"strings"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var (
	installYesFlag bool
)

var installCmd = &cobra.Command{
	Use:   "install [runtime] [version]",
	Short: "Install runtime version(s)",
	Long: `Install a specific version of a runtime, or install all runtimes from .dtvem/runtimes.json.

Single install:
  dtvem install python 3.11.0
  dtvem install node 18.16.0

Bulk install (reads .dtvem/runtimes.json):
  dtvem install
  dtvem install --yes    # Skip confirmation prompt`,
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
}

// installSingle installs a single runtime/version
func installSingle(runtimeName, version string) {
	ui.Debug("Installing single runtime: %s version %s", runtimeName, version)

	provider, err := runtime.Get(runtimeName)
	if err != nil {
		ui.Debug("Provider lookup failed: %v", err)
		ui.Error("%v", err)
		ui.Info("Available runtimes: %v", runtime.List())
		return
	}

	ui.Debug("Using provider: %s (%s)", provider.Name(), provider.DisplayName())

	if err := provider.Install(version); err != nil {
		ui.Debug("Installation failed: %v", err)
		ui.Error("%v", err)
		return
	}

	ui.Success("Successfully installed %s %s", provider.DisplayName(), version)

	// Auto-set global version if no global version is currently configured
	autoSetGlobalIfNeeded(provider, version)
}

// autoSetGlobalIfNeeded sets the installed version as global if no global version exists
func autoSetGlobalIfNeeded(provider runtime.Provider, version string) {
	currentGlobal, err := provider.GlobalVersion()
	if err != nil || currentGlobal != "" {
		// Either an error occurred or a global version is already set
		ui.Debug("Global version check: current=%q, err=%v", currentGlobal, err)
		return
	}

	// No global version configured, auto-set it
	if err := provider.SetGlobalVersion(version); err != nil {
		ui.Debug("Failed to auto-set global version: %v", err)
		ui.Warning("Could not auto-set global version: %v", err)
		return
	}

	ui.Info("Set as global version (first install)")
}

// installBulk installs all runtimes from .dtvem/runtimes.json
// installTask represents a runtime version to be installed
type installTask struct {
	runtimeName      string
	version          string
	provider         runtime.Provider
	alreadyInstalled bool
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

		tasks = append(tasks, installTask{
			runtimeName:      runtimeName,
			version:          version,
			provider:         provider,
			alreadyInstalled: alreadyInstalled,
		})
	}

	return tasks
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
		if task.alreadyInstalled {
			ui.Info("  ✓ %s %s (already installed)", task.provider.DisplayName(), task.version)
			alreadyInstalled++
		} else {
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

		if err := task.provider.Install(task.version); err != nil {
			ui.Error("Failed to install %s %s: %v", task.provider.DisplayName(), task.version, err)
			failures++
			failureList = append(failureList, fmt.Sprintf("%s %s", task.provider.DisplayName(), task.version))
		} else {
			ui.Success("Installed %s %s", task.provider.DisplayName(), task.version)
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
		return
	}

	ui.Info("Found config: %s", configPath)

	runtimes, err := config.ReadAllRuntimes(configPath)
	if err != nil {
		ui.Error("Failed to read config file: %v", err)
		return
	}

	if len(runtimes) == 0 {
		ui.Warning("No runtimes found in config file")
		return
	}

	// Build install tasks
	tasks := buildInstallTasks(runtimes)

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
}
