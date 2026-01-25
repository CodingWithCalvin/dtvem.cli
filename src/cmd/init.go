package cmd

import (
	"runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/spf13/cobra"
)

var (
	initYes  bool
	initUser bool
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize dtvem (setup directories and PATH)",
	Long: `Initialize dtvem by creating necessary directories and configuring your PATH.

This command:
  - Creates the ~/.dtvem directory structure
  - Adds ~/.dtvem/shims to your PATH (with your permission)

Options:
  --user    Use User PATH instead of System PATH on Windows (no admin required)
            Note: System-installed runtimes will take priority over dtvem shims

Run this command after installing dtvem for the first time.

Example:
  dtvem init
  dtvem init --user    # Windows: use User PATH (no admin)`,
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

		// Determine install type and check for switching
		userInstall := determineInstallType(cmd)
		previousSettings, _ := config.LoadSettings()
		isSwitching := cmd.Flags().Changed("user") && previousSettings != nil &&
			((userInstall && previousSettings.InstallType == config.InstallTypeSystem) ||
				(!userInstall && previousSettings.InstallType == config.InstallTypeUser))

		// Warn about switching install types on Windows
		if isSwitching && runtime.GOOS == constants.OSWindows {
			warnAboutInstallTypeSwitch(userInstall, previousSettings.InstallType)
		}

		// Setup PATH - AddToPath handles checking position and moving if needed
		shimsDir := path.ShimsDir()

		if err := path.AddToPath(shimsDir, initYes, userInstall); err != nil {
			ui.Error("Failed to configure PATH: %v", err)
			ui.Info("You can manually add %s to your PATH", shimsDir)
			return
		}

		// Save settings for future reference
		installType := config.InstallTypeSystem
		if userInstall {
			installType = config.InstallTypeUser
		}
		settings := &config.Settings{InstallType: installType}
		if err := config.SaveSettings(settings); err != nil {
			ui.Warning("Failed to save settings: %v", err)
		}

		ui.Success("dtvem initialized successfully!")

		// Show reminder for user-level installations on Windows
		if userInstall && runtime.GOOS == constants.OSWindows {
			ui.Info("")
			ui.Warning("Note: Using User PATH. System-installed runtimes may take priority.")
			ui.Info("Run 'dtvem init' as administrator for system-level PATH if needed.")
		}

		ui.Info("\nNext steps:")
		ui.Info("  1. Restart your terminal (required for PATH changes)")
		ui.Info("  2. Run: dtvem install <runtime> <version>")
		ui.Info("  3. Run: dtvem global <runtime> <version>")
	},
}

// determineInstallType determines whether to use user-level or system-level installation.
// Priority: flag > saved settings > default (system)
func determineInstallType(cmd *cobra.Command) bool {
	// If --user flag was explicitly set, use it
	if cmd.Flags().Changed("user") {
		return initUser
	}

	// Check saved settings
	settings, err := config.LoadSettings()
	if err == nil && settings.InstallType == config.InstallTypeUser {
		return true
	}

	// Default to system install
	return false
}

// warnAboutInstallTypeSwitch warns the user about switching install types
// and provides instructions for cleaning up the old PATH entry.
func warnAboutInstallTypeSwitch(toUser bool, previousType config.InstallType) {
	shimsDir := path.ShimsDir()

	ui.Warning("Switching install type from %s to %s", previousType, map[bool]string{true: "user", false: "system"}[toUser])
	ui.Info("")

	if toUser {
		// Switching from system to user
		ui.Info("Your previous system-level PATH entry may still exist.")
		ui.Info("To avoid conflicts, you may want to remove the old System PATH entry:")
		ui.Info("")
		ui.Info("  Manual removal steps:")
		ui.Info("  1. Open System Properties > Environment Variables")
		ui.Info("  2. Under 'System variables', select 'Path' and click 'Edit'")
		ui.Info("  3. Remove the entry: %s", ui.Highlight(shimsDir))
		ui.Info("  4. Click OK to save")
		ui.Info("")
		ui.Info("  Or run as administrator:")
		ui.Info("    dtvem init   (without --user)")
		ui.Info("  This will move the entry to System PATH properly.")
	} else {
		// Switching from user to system
		ui.Info("Your previous user-level PATH entry may still exist.")
		ui.Info("To avoid conflicts, you may want to remove the old User PATH entry:")
		ui.Info("")
		ui.Info("  Manual removal steps:")
		ui.Info("  1. Open System Properties > Environment Variables")
		ui.Info("  2. Under 'User variables', select 'Path' and click 'Edit'")
		ui.Info("  3. Remove the entry: %s", ui.Highlight(shimsDir))
		ui.Info("  4. Click OK to save")
	}
	ui.Info("")
}

func init() {
	initCmd.Flags().BoolVarP(&initYes, "yes", "y", false, "Skip confirmation prompts")
	initCmd.Flags().BoolVar(&initUser, "user", false, "Use User PATH instead of System PATH (Windows: no admin required)")
	rootCmd.AddCommand(initCmd)
}
