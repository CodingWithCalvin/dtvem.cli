package cmd

import (
	"fmt"

	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
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
			showAllVersions()
		} else {
			showSingleVersion(args[0])
		}
	},
}

// showAllVersions displays all configured runtimes and prompts to install missing ones
func showAllVersions() {
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
	ui.Header("Currently active versions:")
	var missing []runtimeStatus
	for _, rs := range configured {
		if rs.installed {
			fmt.Printf("  %s: %s\n", ui.Highlight(rs.provider.DisplayName()), ui.HighlightVersion(rs.version))
		} else {
			ui.Warning("%s: %s (not installed)", rs.provider.DisplayName(), rs.version)
			missing = append(missing, rs)
		}
	}

	// Prompt to install missing versions
	if len(missing) > 0 {
		fmt.Println()
		if ui.PromptInstallMissing(missing) {
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

// showSingleVersion displays a single runtime version and prompts to install if missing
func showSingleVersion(runtimeName string) {
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
	if installed {
		fmt.Printf("%s: %s\n", ui.Highlight(provider.DisplayName()), ui.HighlightVersion(version))
		return
	}

	// Not installed - show with warning and prompt
	ui.Warning("%s: %s (not installed)", provider.DisplayName(), version)
	fmt.Println()
	if ui.PromptInstall(provider.DisplayName(), version) {
		if err := provider.Install(version); err != nil {
			ui.Error("Failed to install %s %s: %v", provider.DisplayName(), version, err)
			return
		}
		ui.Success("%s %s installed successfully", provider.DisplayName(), version)
	}
}

func init() {
	rootCmd.AddCommand(currentCmd)
}
