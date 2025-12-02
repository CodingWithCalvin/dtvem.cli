package cmd

import (
	"fmt"

	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

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
			// Show all active versions
			ui.Header("Currently active versions:")
			providers := runtime.GetAll()

			if len(providers) == 0 {
				ui.Info("No runtime providers registered")
				return
			}

			for _, provider := range providers {
				version, err := provider.CurrentVersion()
				if err != nil {
					fmt.Printf("  %s: %v\n", provider.DisplayName(), err)
				} else {
					fmt.Printf("  %s: %s\n", ui.Highlight(provider.DisplayName()), ui.HighlightVersion(version))
				}
			}
		} else {
			// Show specific runtime version
			runtimeName := args[0]

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

			fmt.Printf("%s: %s\n", ui.Highlight(provider.DisplayName()), ui.HighlightVersion(version))
		}
	},
}

func init() {
	rootCmd.AddCommand(currentCmd)
}
