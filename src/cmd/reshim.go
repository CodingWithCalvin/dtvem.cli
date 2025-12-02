package cmd

import (
	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/shim"
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
		ui.Header("Regenerating shims...")

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

		// Regenerate shims with spinner
		spinner := ui.NewSpinner("Regenerating shims...")
		spinner.Start()

		if err := manager.Rehash(); err != nil {
			spinner.Error("Failed to regenerate shims")
			ui.Error("%v", err)
			return
		}

		spinner.Success("Shims regenerated successfully")
	},
}

func init() {
	rootCmd.AddCommand(reshimCmd)
}
