package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/spf13/cobra"
)

var uninstallYes bool

var uninstallCmd = &cobra.Command{
	Use:   "uninstall <runtime> <version>",
	Short: "Uninstall a specific runtime version",
	Long: `Uninstall a runtime version from dtvem.

This command removes a specific version of a runtime from your system.
The version directory and all its contents will be deleted.

Safety features:
  - Cannot uninstall the global version while other versions are installed
    (set a different global version first)
  - Uninstalling the only installed version clears the global version, so
    the runtime's commands fall back to your system PATH
  - Prompts for confirmation before deletion
  - Automatically regenerates shims after uninstall

Examples:
  dtvem uninstall python 3.11.0
  dtvem uninstall node 18.16.0
  dtvem uninstall node 18.16.0 --yes    # Skip confirmation prompt`,
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

		// The global version can only go while nothing else could replace
		// it: with other versions installed the user picks the new global
		// first, and with none the global setting is cleared afterwards.
		// Install sets the first version as global, so refusing outright
		// would leave the last version impossible to remove.
		clearGlobal := false
		globalVersion, err := provider.GlobalVersion()
		if err == nil && globalVersion == version {
			installed, err := provider.ListInstalled()
			if err != nil {
				ui.Error("Could not list installed %s versions: %v", provider.DisplayName(), err)
				return
			}
			if others := otherVersions(installed, version); len(others) > 0 {
				ui.Error("Cannot uninstall the global version while other versions are installed")
				ui.Info("Current global version: v%s", globalVersion)
				ui.Info("Other installed versions: %s", strings.Join(others, ", "))
				ui.Info("Set a different global version first: dtvem global %s <version>", runtimeName)
				return
			}
			clearGlobal = true
		}

		// Check if a project file pins this version for the working directory
		if source := localPinSource(provider.Name(), version); source != "" {
			ui.Warning("This is the currently active local version in this directory")
			ui.Info("Local version file: %s", source)
			ui.Info("You may need to update or remove it after uninstalling")
		}

		if clearGlobal {
			ui.Warning("v%s is the global %s version and the only one installed", version, provider.DisplayName())
			ui.Info("The global %s version will be cleared", provider.DisplayName())
		}

		// Prompt for confirmation (unless --yes flag is provided)
		if !uninstallYes {
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

		if clearGlobal {
			if err := config.UnsetGlobalVersion(provider.Name()); err != nil {
				ui.Warning("Could not clear the global %s version: %v", provider.DisplayName(), err)
				ui.Info("Remove the %q entry from %s", provider.Name(), config.GlobalConfigPath())
			} else {
				ui.Info("Cleared the global %s version", provider.DisplayName())
			}
		}

		// Regenerate shims
		shimSpinner := ui.NewSpinner("Regenerating shims...")
		shimSpinner.Start()

		manager, err := shim.NewManager()
		if err != nil {
			shimSpinner.Warning("Could not regenerate shims")
			ui.Warning("You may need to run 'dtvem reshim' manually")
		} else {
			if _, err := manager.Rehash(); err != nil {
				shimSpinner.Warning("Could not regenerate shims")
				ui.Warning("You may need to run 'dtvem reshim' manually")
			} else {
				shimSpinner.Success("Shims regenerated")
			}
		}

		ui.Success("Successfully uninstalled %s v%s", provider.DisplayName(), version)
	},
}

// localPinSource returns the file that sets version for runtimeName in the
// working directory or one of its parents, or "" when none does. The global
// config does not count: provider.LocalVersion falls back to it, which made
// the global version look pinned locally everywhere.
func localPinSource(runtimeName, version string) string {
	res, err := config.Resolve(runtimeName)
	if err != nil || res.Version != version || res.Source == config.GlobalConfigPath() {
		return ""
	}
	return res.Source
}

// otherVersions returns the installed versions other than version, in the
// order given.
func otherVersions(installed []runtime.InstalledVersion, version string) []string {
	var others []string
	for _, iv := range installed {
		if iv.Version.Raw != version {
			others = append(others, iv.Version.Raw)
		}
	}
	return others
}

func init() {
	uninstallCmd.Flags().BoolVarP(&uninstallYes, "yes", "y", false, "Skip confirmation prompt")
	rootCmd.AddCommand(uninstallCmd)
}
