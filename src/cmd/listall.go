package cmd

import (
	"fmt"
	"strings"

	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var listAllCmd = &cobra.Command{
	Use:   "list-all <runtime>",
	Short: "List all available versions of a runtime",
	Long: `Display all available versions of a runtime that can be installed.

This command queries official sources to show all versions available for download.
Installed versions are marked with a ✓ indicator.

Examples:
  dtvem list-all python
  dtvem list-all node
  dtvem list-all python --filter 3.11`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]
		filter, _ := cmd.Flags().GetString("filter")

		// Get the provider
		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("Unknown runtime: %s", runtimeName)
			ui.Info("Available runtimes: %v", runtime.List())
			return
		}

		ui.Header("Available %s versions", provider.DisplayName())
		fmt.Println()
		ui.Info("Fetching available versions...")

		// Get available versions
		available, err := provider.ListAvailable()
		if err != nil {
			ui.Error("Failed to fetch available versions: %v", err)
			return
		}

		if len(available) == 0 {
			ui.Warning("No versions found")
			return
		}

		// Get installed versions for comparison
		installed, err := provider.ListInstalled()
		if err != nil {
			ui.Warning("Could not check installed versions: %v", err)
			installed = []runtime.InstalledVersion{} // Continue without installed info
		}

		// Create a map of installed versions for quick lookup
		installedMap := make(map[string]bool)
		for _, v := range installed {
			installedMap[v.Version.Raw] = true
		}

		// Filter versions if requested
		filteredVersions := available
		if filter != "" {
			filteredVersions = []runtime.AvailableVersion{}
			for _, v := range available {
				if strings.Contains(v.Version.Raw, filter) {
					filteredVersions = append(filteredVersions, v)
				}
			}
		}

		if len(filteredVersions) == 0 {
			ui.Warning("No versions match filter: %s", filter)
			return
		}

		fmt.Println()
		ui.Success("Found %d version(s)", len(filteredVersions))
		if filter != "" {
			ui.Info("Filtered by: %s", filter)
		}
		fmt.Println()

		// Display versions
		count := 0
		maxDisplay := 50 // Limit display to avoid overwhelming output

		for _, v := range filteredVersions {
			if count >= maxDisplay {
				ui.Info("... and %d more (use --filter to narrow results)", len(filteredVersions)-count)
				break
			}

			// Check if installed
			marker := " "
			if installedMap[v.Version.Raw] {
				marker = "✓"
			}

			// Display version with marker
			fmt.Printf("  %s %s", marker, ui.HighlightVersion(v.Version.Raw))

			// Add notes if available
			if v.Notes != "" {
				fmt.Printf(" (%s)", v.Notes)
			}

			fmt.Println()
			count++
		}

		fmt.Println()
		ui.Info("Install a version with: dtvem install %s <version>", runtimeName)
		if len(filteredVersions) > maxDisplay {
			ui.Info("Use --filter to narrow down results")
		}
	},
}

func init() {
	listAllCmd.Flags().StringP("filter", "f", "", "Filter versions by substring (e.g., '3.11' for Python 3.11.x)")
	rootCmd.AddCommand(listAllCmd)
}
