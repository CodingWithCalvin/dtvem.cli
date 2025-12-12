package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/shim"
	"github.com/dtvem/dtvem/src/internal/tui"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var reshimCmd = &cobra.Command{
	Use:   "reshim",
	Short: "Regenerate shim binaries",
	Long: `Regenerate shim binaries for all installed runtime versions.

This command scans all installed runtimes and creates shims for their executables.
Run this command after installing new versions or if shims become corrupted.

Example:
  dtvem reshim`,
	Run: func(cmd *cobra.Command, args []string) {
		// Ensure directories exist
		if err := config.EnsureDirectories(); err != nil {
			ui.Error("Failed to create directories: %v", err)
			return
		}

		// Create shim manager
		manager, err := shim.NewManager()
		if err != nil {
			ui.Error("%v", err)
			ui.Info("Note: Make sure dtvem-shim executable is built and available")
			return
		}

		// Collect display names as we process
		displayNames := make(map[string]string)

		// Regenerate shims with per-runtime progress
		result, err := manager.RehashWithCallback(func(runtimeName, displayName string) {
			displayNames[runtimeName] = displayName
			ui.Info("Regenerating shims for %s...", displayName)
		})

		if err != nil {
			fmt.Println()
			ui.Error("%v", err)
			return
		}

		fmt.Println()

		// Display results in a table
		table := tui.NewTable("Runtime", "Shims")
		table.SetTitle("Shims Created")

		// Sort runtime names for consistent output
		runtimeNames := make([]string, 0, len(result.ShimsByRuntime))
		for name := range result.ShimsByRuntime {
			runtimeNames = append(runtimeNames, name)
		}
		sort.Strings(runtimeNames)

		for _, runtimeName := range runtimeNames {
			shims := result.ShimsByRuntime[runtimeName]
			sort.Strings(shims)

			// Get display name from provider
			displayName := runtimeName
			if provider, err := runtime.Get(runtimeName); err == nil {
				displayName = provider.DisplayName()
			} else if len(displayName) > 0 {
				// Capitalize first letter as fallback
				displayName = strings.ToUpper(displayName[:1]) + displayName[1:]
			}

			// Format shims list (truncate if too long)
			shimList := strings.Join(shims, ", ")
			if len(shimList) > 60 {
				shimList = shimList[:57] + "..."
			}

			table.AddRow(displayName, shimList)
		}

		fmt.Println(table.Render())
		fmt.Println()
		ui.Success("Created %d shims for %d runtime(s)", result.TotalShims, len(result.ShimsByRuntime))
	},
}

func init() {
	rootCmd.AddCommand(reshimCmd)
}
