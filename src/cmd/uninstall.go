package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/shim"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <runtime> <version>",
	Short: "Uninstall a specific runtime version",
	Long: `Uninstall a runtime version from dtvem.

This command removes a specific version of a runtime from your system.
The version directory and all its contents will be deleted.

Safety features:
  - Cannot uninstall the currently active global version
  - Prompts for confirmation before deletion
  - Automatically regenerates shims after uninstall

Examples:
  dtvem uninstall python 3.11.0
  dtvem uninstall node 18.16.0`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]
		version := args[1]

		// Strip 'v' prefix if present
		version = strings.TrimPrefix(version, "v")

		// Get the runtime provider
		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("%v", err)
			ui.Info("Available runtimes: %v", runtime.List())
			return
		}

		ui.Header("Uninstalling %s v%s...", provider.DisplayName(), version)

		// Check if version is installed
		versionPath := config.RuntimeVersionPath(runtimeName, version)
		if _, err := os.Stat(versionPath); os.IsNotExist(err) {
			ui.Error("Version %s is not installed", version)
			ui.Info("Run 'dtvem list %s' to see installed versions", runtimeName)
			return
		}

		// Check if this is the currently active global version
		globalVersion, err := provider.GlobalVersion()
		if err == nil && globalVersion == version {
			ui.Error("Cannot uninstall the currently active global version")
			ui.Info("Current global version: v%s", globalVersion)
			ui.Info("Set a different global version first: dtvem global %s <version>", runtimeName)
			return
		}

		// Check if this is the currently active local version
		localVersion, err := provider.LocalVersion()
		if err == nil && localVersion == version {
			ui.Warning("This is the currently active local version in this directory")
			ui.Info("Local version file: dtvem.config.json")
			ui.Info("You may need to update or remove it after uninstalling")
		}

		// Prompt for confirmation
		fmt.Printf("\n")
		ui.Warning("This will permanently delete:")
		ui.Info("  %s", versionPath)
		fmt.Printf("\nAre you sure you want to uninstall %s v%s? [y/N]: ", provider.DisplayName(), version)

		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != constants.ResponseY && response != constants.ResponseYes {
			ui.Info("Uninstall canceled")
			return
		}

		// Remove the version directory
		spinner := ui.NewSpinner(fmt.Sprintf("Removing %s v%s...", provider.DisplayName(), version))
		spinner.Start()

		if err := os.RemoveAll(versionPath); err != nil {
			spinner.Error("Failed to remove version")
			ui.Error("Error: %v", err)
			return
		}

		spinner.Success(fmt.Sprintf("%s v%s removed", provider.DisplayName(), version))

		// Regenerate shims
		shimSpinner := ui.NewSpinner("Regenerating shims...")
		shimSpinner.Start()

		manager, err := shim.NewManager()
		if err != nil {
			shimSpinner.Warning("Could not regenerate shims")
			ui.Warning("You may need to run 'dtvem reshim' manually")
		} else {
			if err := manager.Rehash(); err != nil {
				shimSpinner.Warning("Could not regenerate shims")
				ui.Warning("You may need to run 'dtvem reshim' manually")
			} else {
				shimSpinner.Success("Shims regenerated")
			}
		}

		ui.Success("Successfully uninstalled %s v%s", provider.DisplayName(), version)
	},
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
