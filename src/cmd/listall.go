package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/tui"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
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
		limit, _ := cmd.Flags().GetInt("limit")

		// Get the provider
		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("Unknown runtime: %s", runtimeName)
			ui.Info("Available runtimes: %v", runtime.List())
			return
		}

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

		// Get global and local versions for indicators
		globalVersion, _ := provider.GlobalVersion()
		localVersion, _ := config.LocalVersion(runtimeName)

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

		total := len(filteredVersions)
		offset := 0
		reader := bufio.NewReader(os.Stdin)

		for {
			// Calculate how many to show this page
			remaining := total - offset
			pageSize := limit
			if remaining < pageSize {
				pageSize = remaining
			}

			// Create table for this page
			table := tui.NewTable("", "Version", "Status", "Notes")
			table.SetTitle(provider.DisplayName())

			for i := 0; i < pageSize; i++ {
				v := filteredVersions[offset+i]
				version := v.Version.Raw

				// Check if installed
				marker := ""
				if installedMap[version] {
					marker = tui.CheckMark
				}

				// Get status (global/local indicators)
				status := getVersionStatus(version, globalVersion, localVersion)

				table.AddRow(marker, version, status, v.Notes)
			}

			fmt.Println()
			fmt.Println(table.Render())

			offset += pageSize
			remaining = total - offset

			// Show progress and prompt for more if there are more versions
			if remaining > 0 {
				ui.Printf("Showing %d of %d. Press Enter for more (q to quit): ", offset, total)
				input, _ := reader.ReadString('\n')
				input = strings.TrimSpace(strings.ToLower(input))
				if input == "q" || input == "quit" {
					break
				}
			} else {
				// All versions shown
				fmt.Println()
				ui.Success("Showing all %d version(s)", total)
				break
			}
		}

		fmt.Println()
		ui.Info("Install a version with: dtvem install %s <version>", runtimeName)
	},
}

func init() {
	listAllCmd.Flags().StringP("filter", "f", "", "Filter versions by substring (e.g., '3.11' for Python 3.11.x)")
	listAllCmd.Flags().IntP("limit", "l", 50, "Number of versions to show per page")
	rootCmd.AddCommand(listAllCmd)
}
