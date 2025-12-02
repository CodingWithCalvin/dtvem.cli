package cmd

import (
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list <runtime>",
	Short: "List installed versions of a runtime",
	Long: `List all installed versions of a specific runtime.

Examples:
  dtvem list python
  dtvem list node`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]

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

		for _, v := range versions {
			ui.Printf("  %s\n", ui.HighlightVersion(v.String()))
		}
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
