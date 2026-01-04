package cmd

import (
	"fmt"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/tui"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/spf13/cobra"
)

var (
	currentYes       bool
	currentNoInstall bool
)

// runtimeStatus holds the status of a configured runtime
type runtimeStatus struct {
	provider  runtime.Provider
	version   string
	installed bool
}

var currentCmd = &cobra.Command{
	Use:   "current [runtime]",
	Short: "Show the currently active version(s)",
	Long: `Show the currently active version for a specific runtime or all runtimes.
The active version is determined by checking local settings first, then global settings.

Examples:
  dtvem current           # Show all active versions
  dtvem current python    # Show active Python version
  dtvem current node      # Show active Node.js version`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			showAllVersions(currentYes, currentNoInstall)
		} else {
			showSingleVersion(args[0], currentYes, currentNoInstall)
		}
	},
}

// showAllVersions displays all configured runtimes and prompts to install missing ones.
// If noInstall is true, install prompts are skipped entirely.
// If yes is true, install prompts are auto-accepted.
func showAllVersions(yes, noInstall bool) {
	providers := runtime.GetAll()

	if len(providers) == 0 {
		ui.Info("No runtime providers registered")
		return
	}

	// Collect status for all configured runtimes
	var configured []runtimeStatus
	for _, provider := range providers {
		version, err := provider.CurrentVersion()
		if err != nil {
			// Not configured - skip it
			continue
		}
		installed, _ := provider.IsInstalled(version)
		configured = append(configured, runtimeStatus{
			provider:  provider,
			version:   version,
			installed: installed,
		})
	}

	if len(configured) == 0 {
		ui.Info("No runtimes configured")
		return
	}

	// Display all configured versions
	table := tui.NewTable("Runtime", "Version", "Status")
	table.SetTitle("Active Versions")
	var missing []runtimeStatus

	for _, rs := range configured {
		if rs.installed {
			table.AddActiveRow(rs.provider.DisplayName(), rs.version, tui.CheckMark+" installed")
		} else {
			table.AddRow(rs.provider.DisplayName(), rs.version, tui.CrossMark+" not installed")
			missing = append(missing, rs)
		}
	}

	fmt.Println(table.Render())

	// Skip install prompts if --no-install flag is set
	if noInstall {
		return
	}

	// Prompt to install missing versions
	if len(missing) > 0 {
		fmt.Println()
		shouldInstall := yes || ui.PromptInstallMissing(missing)
		if shouldInstall {
			for _, rs := range missing {
				ui.Info("Installing %s %s...", rs.provider.DisplayName(), rs.version)
				if err := rs.provider.Install(rs.version); err != nil {
					ui.Error("Failed to install %s %s: %v", rs.provider.DisplayName(), rs.version, err)
				} else {
					ui.Success("%s %s installed successfully", rs.provider.DisplayName(), rs.version)
				}
			}
		}
	}
}

// showSingleVersion displays a single runtime version and prompts to install if missing.
// If noInstall is true, install prompts are skipped entirely.
// If yes is true, install prompts are auto-accepted.
func showSingleVersion(runtimeName string, yes, noInstall bool) {
	provider, err := runtime.Get(runtimeName)
	if err != nil {
		ui.Error("%v", err)
		ui.Info("Available runtimes: %v", runtime.List())
		return
	}

	version, err := provider.CurrentVersion()
	if err != nil {
		ui.Error("%v", err)
		return
	}

	installed, _ := provider.IsInstalled(version)

	table := tui.NewTable("Runtime", "Version", "Status")
	if installed {
		table.AddActiveRow(provider.DisplayName(), version, tui.CheckMark+" installed")
		fmt.Println(table.Render())
		return
	}

	// Not installed - show with warning and prompt
	table.AddRow(provider.DisplayName(), version, tui.CrossMark+" not installed")
	fmt.Println(table.Render())

	// Skip install prompts if --no-install flag is set
	if noInstall {
		return
	}

	fmt.Println()
	shouldInstall := yes || ui.PromptInstall(provider.DisplayName(), version)
	if shouldInstall {
		if err := provider.Install(version); err != nil {
			ui.Error("Failed to install %s %s: %v", provider.DisplayName(), version, err)
			return
		}
		ui.Success("%s %s installed successfully", provider.DisplayName(), version)
	}
}

func init() {
	currentCmd.Flags().BoolVarP(&currentYes, "yes", "y", false, "Automatically install missing versions without prompting")
	currentCmd.Flags().BoolVarP(&currentNoInstall, "no-install", "n", false, "Skip install prompts entirely")
	rootCmd.AddCommand(currentCmd)
}
