package cmd

import (
	"fmt"
	"os"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var whereCmd = &cobra.Command{
	Use:   "where <runtime> [version]",
	Short: "Show the installation directory for a runtime version",
	Long: `Display the full path to where a runtime version is installed.

If no version is specified, shows the location of the currently active version.

Examples:
  dtvem where python 3.11.0
  dtvem where node 18.16.0
  dtvem where python          # Shows current version location`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]

		// Get the provider
		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("Unknown runtime: %s", runtimeName)
			ui.Info("Available runtimes: %v", runtime.List())
			return
		}

		var version string

		// If version not specified, use current version
		if len(args) == 1 {
			version, err = provider.CurrentVersion()
			if err != nil {
				ui.Error("No version configured for %s", runtimeName)
				ui.Info("Set a version with: dtvem global %s <version>", runtimeName)
				ui.Info("Or specify a version: dtvem where %s <version>", runtimeName)
				return
			}
			ui.Info("Using current version: %s", ui.HighlightVersion(version))
			fmt.Println()
		} else {
			version = args[1]
			// Strip 'v' prefix if present
			if len(version) > 0 && (version[0] == 'v' || version[0] == 'V') {
				version = version[1:]
			}
		}

		// Check if version is installed
		installed, err := provider.IsInstalled(version)
		if err != nil {
			ui.Error("Failed to check if version is installed: %v", err)
			return
		}
		if !installed {
			ui.Error("Version %s is not installed", version)
			ui.Info("Install it with: dtvem install %s %s", runtimeName, version)
			return
		}

		// Get the installation path
		installPath := config.RuntimeVersionPath(runtimeName, version)

		// Verify the path exists
		if _, err := os.Stat(installPath); os.IsNotExist(err) {
			ui.Error("Installation directory not found: %s", installPath)
			ui.Warning("Version may be corrupted or partially installed")
			return
		}

		// Display the information
		ui.Header("%s %s", provider.DisplayName(), ui.HighlightVersion(version))
		fmt.Println()
		ui.Success("Installed at: %s", installPath)
		fmt.Println()

		// Show additional information about the installation
		ui.Info("Contents:")
		entries, err := os.ReadDir(installPath)
		if err != nil {
			ui.Warning("Unable to read directory contents: %v", err)
			return
		}

		// Show top-level directories/files
		count := 0
		for _, entry := range entries {
			if count >= 10 {
				ui.Info("  ... and %d more", len(entries)-count)
				break
			}
			if entry.IsDir() {
				ui.Info("  📁 %s/", entry.Name())
			} else {
				ui.Info("  📄 %s", entry.Name())
			}
			count++
		}
	},
}

func init() {
	rootCmd.AddCommand(whereCmd)
}
