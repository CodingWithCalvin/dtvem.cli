package cmd

import (
	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/path"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize dtvem (setup directories and PATH)",
	Long: `Initialize dtvem by creating necessary directories and configuring your PATH.

This command:
  - Creates the ~/.dtvem directory structure
  - Adds ~/.dtvem/shims to your PATH (with your permission)

Run this command after installing dtvem for the first time.

Example:
  dtvem init`,
	Run: func(cmd *cobra.Command, args []string) {
		ui.Header("Initializing dtvem...")

		// Ensure directories exist
		spinner := ui.NewSpinner("Creating directories...")
		spinner.Start()

		if err := config.EnsureDirectories(); err != nil {
			spinner.Error("Failed to create directories")
			ui.Error("%v", err)
			return
		}

		spinner.Success("Directories created")

		// Setup PATH - AddToPath handles checking position and moving if needed
		shimsDir := path.ShimsDir()

		if err := path.AddToPath(shimsDir); err != nil {
			ui.Error("Failed to configure PATH: %v", err)
			ui.Info("You can manually add %s to your PATH", shimsDir)
			return
		}

		ui.Success("dtvem initialized successfully!")
		ui.Info("\nNext steps:")
		ui.Info("  1. Restart your terminal (required for PATH changes)")
		ui.Info("  2. Run: dtvem install <runtime> <version>")
		ui.Info("  3. Run: dtvem global <runtime> <version>")
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
