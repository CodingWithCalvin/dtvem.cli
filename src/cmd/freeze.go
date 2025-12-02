package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var freezeCmd = &cobra.Command{
	Use:   "freeze",
	Short: "Create .dtvem/runtimes.json from global runtime versions",
	Long: `Create a .dtvem/runtimes.json file in the current directory based on your globally configured runtime versions.

This command will:
  1. Show all globally configured runtimes
  2. Let you select which ones to include
  3. Create .dtvem/runtimes.json with the selected runtimes

Examples:
  dtvem freeze`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		ui.Header("Freeze runtime versions")
		fmt.Println()

		configPath := config.LocalConfigPath()

		// Check if .dtvem/runtimes.json already exists
		if _, err := os.Stat(configPath); err == nil {
			ui.Warning(".dtvem/runtimes.json already exists in this directory")
			fmt.Printf("Overwrite it? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != constants.ResponseY && response != constants.ResponseYes {
				ui.Info("Canceled")
				return
			}
			fmt.Println()
		}

		// Read global config
		globalConfigPath := config.GlobalConfigPath()
		if _, err := os.Stat(globalConfigPath); os.IsNotExist(err) {
			ui.Error("No global configuration found")
			ui.Info("Set global versions first with: dtvem global <runtime> <version>")
			return
		}

		// Parse global config to get all runtimes
		data, err := os.ReadFile(globalConfigPath)
		if err != nil {
			ui.Error("Failed to read global config: %v", err)
			return
		}

		var globalConfig config.RuntimesConfig
		if err := json.Unmarshal(data, &globalConfig); err != nil {
			ui.Error("Failed to parse global config: %v", err)
			return
		}

		if len(globalConfig) == 0 {
			ui.Warning("No global runtimes configured")
			ui.Info("Set global versions first with: dtvem global <runtime> <version>")
			return
		}

		// Display available runtimes
		ui.Success("Found %d globally configured runtime(s):", len(globalConfig))
		fmt.Println()

		runtimeList := make([]struct {
			Name    string
			Version string
		}, 0, len(globalConfig))

		i := 1
		for runtimeName, version := range globalConfig {
			runtimeList = append(runtimeList, struct {
				Name    string
				Version string
			}{Name: runtimeName, Version: version})

			// Get display name if provider exists
			displayName := runtimeName
			if provider, err := runtime.Get(runtimeName); err == nil {
				displayName = provider.DisplayName()
			}

			fmt.Printf("  [%d] %s %s\n", i, displayName, ui.HighlightVersion(version))
			i++
		}

		// Prompt for selection
		fmt.Println()
		fmt.Printf("Select runtimes to include (comma-separated numbers, or 'all'): ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			ui.Info("Canceled")
			return
		}

		// Parse selection
		var selectedRuntimes config.RuntimesConfig

		if strings.ToLower(input) == "all" {
			selectedRuntimes = globalConfig
		} else {
			selectedRuntimes = make(config.RuntimesConfig)
			selections := strings.Split(input, ",")

			for _, sel := range selections {
				sel = strings.TrimSpace(sel)
				index, err := strconv.Atoi(sel)
				if err != nil || index < 1 || index > len(runtimeList) {
					ui.Warning("Invalid selection: %s", sel)
					continue
				}

				rt := runtimeList[index-1]
				selectedRuntimes[rt.Name] = rt.Version
			}
		}

		if len(selectedRuntimes) == 0 {
			ui.Error("No runtimes selected")
			return
		}

		// Ensure .dtvem directory exists
		configDir := config.LocalConfigDir()
		if err := os.MkdirAll(configDir, 0755); err != nil {
			ui.Error("Failed to create .dtvem directory: %v", err)
			return
		}

		// Create config file
		data, err = json.MarshalIndent(selectedRuntimes, "", "  ")
		if err != nil {
			ui.Error("Failed to create config: %v", err)
			return
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			ui.Error("Failed to write config file: %v", err)
			return
		}

		// Success message
		fmt.Println()
		ui.Success("Created .dtvem/runtimes.json with %d runtime(s):", len(selectedRuntimes))
		for name, version := range selectedRuntimes {
			displayName := name
			if provider, err := runtime.Get(name); err == nil {
				displayName = provider.DisplayName()
			}
			ui.Info("  %s %s", displayName, ui.HighlightVersion(version))
		}
	},
}

func init() {
	rootCmd.AddCommand(freezeCmd)
}
