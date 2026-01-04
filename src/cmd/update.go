package cmd

import (
	"fmt"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/manifest"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/tui"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update runtime version manifests",
	Long: `Force refresh the cached runtime version manifests.

This command bypasses the 24-hour cache and fetches fresh manifest data
from the remote server. If the remote server is unavailable, it falls
back to the embedded manifests bundled with dtvem.

Example:
  dtvem update           # Update all runtime manifests
  dtvem update python    # Update only the Python manifest`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get list of runtimes to update
		var runtimes []string
		var err error

		if len(args) > 0 {
			// Specific runtime requested
			runtimes = args
		} else {
			// Update all available runtimes
			runtimes, err = manifest.ListAvailableRuntimes()
			if err != nil {
				ui.Error("Failed to list runtimes: %v", err)
				return
			}
		}

		if len(runtimes) == 0 {
			ui.Warning("No runtimes found to update")
			return
		}

		ui.Info("Updating manifests...")
		fmt.Println()

		// Build results table
		table := tui.NewTable("Runtime", "Versions", "Source")
		table.SetTitle("Manifest Update Results")

		hasErrors := false
		for _, runtime := range runtimes {
			m, fromRemote, err := manifest.ForceRefreshRuntime(runtime)
			if err != nil {
				ui.Error("  %s: %v", runtime, err)
				hasErrors = true
				continue
			}

			source := "embedded"
			if fromRemote {
				source = "remote"
			}

			table.AddRow(runtime, fmt.Sprintf("%d versions", len(m.Versions)), source)
		}

		fmt.Println(table.Render())
		fmt.Println()

		if hasErrors {
			ui.Warning("Some manifests could not be updated")
		} else {
			ui.Success("All manifests updated successfully")
		}
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
