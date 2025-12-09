package cmd

import (
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list [runtime]",
	Short: "List installed versions",
	Long: `List all installed versions of a specific runtime, or all runtimes if none specified.

Examples:
  dtvem list           # List all installed versions
  dtvem list python    # List installed Python versions
  dtvem list node      # List installed Node.js versions`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			listAllRuntimes()
		} else {
			listSingleRuntime(args[0])
		}
	},
}

// listAllRuntimes lists installed versions for all runtimes
func listAllRuntimes() {
	providers := runtime.GetAll()

	if len(providers) == 0 {
		ui.Info("No runtime providers registered")
		return
	}

	ui.Header("Installed versions:")

	hasAny := false
	for _, provider := range providers {
		versions, err := provider.ListInstalled()
		if err != nil {
			ui.Error("  %s: %v", provider.DisplayName(), err)
			continue
		}

		if len(versions) == 0 {
			continue
		}

		hasAny = true
		globalVersion, _ := provider.GlobalVersion()

		ui.Printf("  %s:\n", ui.Highlight(provider.DisplayName()))
		for _, v := range versions {
			if v.String() == globalVersion {
				ui.Printf("    %s (global)\n", ui.HighlightVersion(v.String()))
			} else {
				ui.Printf("    %s\n", ui.HighlightVersion(v.String()))
			}
		}
	}

	if !hasAny {
		ui.Info("No versions installed")
	}
}

// listSingleRuntime lists installed versions for a specific runtime
func listSingleRuntime(runtimeName string) {
	provider, err := runtime.Get(runtimeName)
	if err != nil {
		ui.Error("%v", err)
		ui.Info("Available runtimes: %v", runtime.List())
		return
	}

	ui.Header("Installed %s versions:", provider.DisplayName())

	versions, err := provider.ListInstalled()
	if err != nil {
		ui.Error("%v", err)
		return
	}

	if len(versions) == 0 {
		ui.Info("No versions installed")
		return
	}

	globalVersion, _ := provider.GlobalVersion()

	for _, v := range versions {
		if v.String() == globalVersion {
			ui.Printf("  %s (global)\n", ui.HighlightVersion(v.String()))
		} else {
			ui.Printf("  %s\n", ui.HighlightVersion(v.String()))
		}
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
