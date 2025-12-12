package cmd

import (
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

// setRuntimeVersion is a helper function for setting runtime versions (global or local)
func setRuntimeVersion(runtimeName, version, scope string, setter func(string) error) {
	provider, err := runtime.Get(runtimeName)
	if err != nil {
		ui.Error("%v", err)
		ui.Info("Available runtimes: %v", runtime.List())
		return
	}

	// Validate that the version is installed
	installed, err := provider.IsInstalled(version)
	if err != nil {
		ui.Error("Failed to check if version is installed: %v", err)
		return
	}
	if !installed {
		ui.Error("%s %s is not installed", provider.DisplayName(), version)
		ui.Info("Run 'dtvem list %s' to see installed versions", runtimeName)
		ui.Info("Run 'dtvem install %s %s' to install it first", runtimeName, version)
		return
	}

	ui.Info("Setting %s %s version to %s...", scope, provider.DisplayName(), version)

	if err := setter(version); err != nil {
		ui.Error("%v", err)
		return
	}

	ui.Success("Successfully set %s %s version to %s", scope, provider.DisplayName(), version)
}

var globalCmd = &cobra.Command{
	Use:   "global <runtime> <version>",
	Short: "Set the global default version of a runtime",
	Long: `Set the global default version for a runtime.
This version will be used when no local version is specified.

Examples:
  dtvem global python 3.11.0
  dtvem global node 18.16.0`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]
		version := args[1]

		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("%v", err)
			ui.Info("Available runtimes: %v", runtime.List())
			return
		}

		setRuntimeVersion(runtimeName, version, "global", provider.SetGlobalVersion)
	},
}

func init() {
	rootCmd.AddCommand(globalCmd)
}
