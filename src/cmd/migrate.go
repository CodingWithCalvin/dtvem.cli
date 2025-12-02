package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

	internalRuntime "github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate <runtime>",
	Short: "Migrate existing runtime installations to dtvem",
	Long: `Detect existing installations of a runtime and migrate them to dtvem.

This command scans your system for existing installations (from system packages,
nvm, pyenv, etc.), lets you select which versions to migrate, and installs them
via dtvem's normal installation process.

Examples:
  dtvem migrate node     # Detect and migrate Node.js installations
  dtvem migrate python   # Detect and migrate Python installations`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]

		// Get the runtime provider
		provider, err := internalRuntime.Get(runtimeName)
		if err != nil {
			ui.Error("%v", err)
			ui.Info("Available runtimes: %v", internalRuntime.List())
			return
		}

		spinner := ui.NewSpinner(fmt.Sprintf("Scanning for %s installations...", provider.DisplayName()))
		spinner.Start()

		// Detect existing installations
		detected, err := provider.DetectInstalled()
		if err != nil {
			spinner.Error("Scan failed")
			ui.Error("Error detecting installations: %v", err)
			return
		}

		if len(detected) == 0 {
			spinner.Warning("No installations found")
			ui.Info("Use 'dtvem install %s <version>' to install a version", runtimeName)
			return
		}

		spinner.Success(fmt.Sprintf("Found %d installation(s)", len(detected)))
		fmt.Println()

		// Display detected installations
		for i, dv := range detected {
			validatedMark := ""
			if dv.Validated {
				validatedMark = " " + ui.Highlight("✓")
			}
			fmt.Printf("  [%d] %s  (%s) %s%s\n",
				i+1,
				ui.HighlightVersion("v"+dv.Version),
				ui.Highlight(dv.Source),
				dv.Path,
				validatedMark)
		}

		// Prompt user for selection
		fmt.Printf("\nSelect versions to migrate:\n")
		fmt.Printf("  Enter numbers separated by commas, or 'all' (e.g., 1,3 or all): ")

		reader := bufio.NewReader(os.Stdin)
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			return
		}

		input = strings.TrimSpace(input)
		if input == "" {
			ui.Info("No versions selected. Exiting")
			return
		}

		// Parse selection
		selectedIndices := parseSelection(input, len(detected))
		if len(selectedIndices) == 0 {
			ui.Warning("No valid selections. Exiting")
			return
		}

		// Get selected versions
		selectedVersions := make([]internalRuntime.DetectedVersion, 0)
		for _, idx := range selectedIndices {
			selectedVersions = append(selectedVersions, detected[idx])
		}

		fmt.Println()

		// Migrate each selected version
		successCount := 0
		fmt.Println()
		for _, dv := range selectedVersions {
			ui.Header("Migrating %s v%s...", provider.DisplayName(), dv.Version)

			// Detect global packages from the existing installation
			var globalPackages []string
			ui.Progress("Detecting global packages...")
			packages, err := provider.GlobalPackages(dv.Path)
			if err != nil {
				ui.Warning("Could not detect global packages: %v", err)
				globalPackages = []string{}
			} else {
				globalPackages = packages
				if len(globalPackages) > 0 {
					ui.Info("Found %d global package(s): %s", len(globalPackages), strings.Join(globalPackages, ", "))
				} else {
					ui.Info("No global packages found")
				}
			}

			// TODO: Detect and preserve configuration files/settings
			// For Node.js: Check for .npmrc in installation dir or ~/.npmrc
			// For Python: Check for pip.conf/pip.ini
			// Handle sensitive data (auth tokens) appropriately

			// Call the provider's Install method
			if err := provider.Install(dv.Version); err != nil {
				ui.Error("%v", err)
			} else {
				successCount++

				// Reinstall global packages
				if len(globalPackages) > 0 {
					ui.Progress("Reinstalling %d global package(s)...", len(globalPackages))
					if err := provider.InstallGlobalPackages(dv.Version, globalPackages); err != nil {
						ui.Warning("Failed to reinstall some packages: %v", err)
						if cmd := provider.ManualPackageInstallCommand(globalPackages); cmd != "" {
							ui.Info("You can manually reinstall with:")
							ui.Info("  %s", cmd)
						}
					} else {
						ui.Success("Reinstalled %d global package(s)", len(globalPackages))
					}
				}

				// TODO: Copy/merge configuration files to new installation
				// Ensure settings like registry URLs, proxies, etc. are preserved
			}
			fmt.Println()
		}

		if successCount == len(selectedVersions) {
			ui.Success("Migration complete! %d/%d version(s) installed", successCount, len(selectedVersions))
		} else if successCount > 0 {
			ui.Warning("Migration partially complete: %d/%d version(s) installed", successCount, len(selectedVersions))
		} else {
			ui.Error("Migration failed: 0/%d version(s) installed", len(selectedVersions))
		}

		if successCount == 0 {
			return
		}

		// Optionally set global version
		if successCount > 0 {
			fmt.Println()
			ui.Header("Set global version?")
			for i, dv := range selectedVersions {
				fmt.Printf("  [%d] %s\n", i+1, ui.HighlightVersion("v"+dv.Version))
			}
			fmt.Printf("  [0] None\n")
			fmt.Printf("Select [0]: ")

			input, err = reader.ReadString('\n')
			if err == nil {
				input = strings.TrimSpace(input)
				if input != "" && input != "0" {
					if choice, err := strconv.Atoi(input); err == nil && choice > 0 && choice <= len(selectedVersions) {
						version := selectedVersions[choice-1].Version
						if err := provider.SetGlobalVersion(version); err != nil {
							ui.Error("Error setting global version: %v", err)
						} else {
							ui.Success("Global version set to v%s", version)
						}
					}
				}
			}
		}

		// Prompt to cleanup old installations
		if successCount > 0 {
			fmt.Println()
			promptCleanupOldInstallations(selectedVersions, provider.DisplayName())
		}

		// Show next steps
		fmt.Println()
		ui.Header("Next steps:")
		ui.Info("1. Add ~/.dtvem/shims to your PATH (if not already)")
		ui.Info("2. Run: %s --version", runtimeName)
		ui.Info("3. Consider removing old installations to avoid conflicts")
	},
}

// parseSelection parses user selection input like "1,3,5" or "all"
func parseSelection(input string, maxCount int) []int {
	indices := make([]int, 0, maxCount)

	if strings.ToLower(input) == "all" {
		for i := 0; i < maxCount; i++ {
			indices = append(indices, i)
		}
		return indices
	}

	parts := strings.Split(input, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if num, err := strconv.Atoi(part); err == nil {
			// Convert to 0-indexed
			idx := num - 1
			if idx >= 0 && idx < maxCount {
				indices = append(indices, idx)
			}
		}
	}

	return indices
}

// promptCleanupOldInstallations prompts the user to clean up old installations after successful migration
func promptCleanupOldInstallations(versions []internalRuntime.DetectedVersion, runtimeDisplayName string) {
	ui.Header("Cleanup Old Installations")
	ui.Info("You have successfully migrated to dtvem. Would you like to clean up the old installations?")
	ui.Info("This helps prevent PATH conflicts and version confusion.")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	removedCount := 0
	skippedCount := 0

	for _, dv := range versions {
		fmt.Printf("Old installation: %s %s\n", ui.HighlightVersion("v"+dv.Version), ui.Highlight("("+dv.Source+")"))
		fmt.Printf("  Location: %s\n", dv.Path)

		// Get uninstall instructions/command based on source
		instructions, command, automatable := getUninstallInstructions(dv, runtimeDisplayName)

		if automatable {
			fmt.Printf("\nRemove this installation? [y/N]: ")
			input, err := reader.ReadString('\n')
			if err != nil || strings.ToLower(strings.TrimSpace(input)) != "y" {
				skippedCount++
				ui.Warning("Skipped. You can manually remove it later with:")
				ui.Info("  %s", command)
				fmt.Println()
				continue
			}

			// Attempt to execute the uninstall command
			ui.Progress("Removing %s v%s from %s...", runtimeDisplayName, dv.Version, dv.Source)
			if err := executeUninstallCommand(command); err != nil {
				ui.Error("Failed to remove: %v", err)
				ui.Info("You can manually remove it with:")
				ui.Info("  %s", command)
				skippedCount++
			} else {
				ui.Success("Removed %s v%s from %s", runtimeDisplayName, dv.Version, dv.Source)
				removedCount++
			}
		} else {
			// System installs - provide instructions only
			ui.Warning("Manual removal required")
			ui.Info("%s", instructions)
			skippedCount++
		}
		fmt.Println()
	}

	// Summary
	if removedCount > 0 {
		ui.Success("Removed %d old installation(s)", removedCount)
	}
	if skippedCount > 0 {
		ui.Warning("PATH Conflict Warning")
		ui.Info("You have %d old installation(s) remaining", skippedCount)
		ui.Info("These may conflict with dtvem-managed versions if they appear earlier in your PATH")
		ui.Info("Consider removing them manually to avoid confusion")
	}
}

// getUninstallInstructions returns instructions, command, and whether it's automatable
func getUninstallInstructions(dv internalRuntime.DetectedVersion, runtimeDisplayName string) (instructions string, command string, automatable bool) {
	version := dv.Version
	source := strings.ToLower(dv.Source)

	switch source {
	case "nvm":
		return "",
			fmt.Sprintf("nvm uninstall %s", version),
			true
	case "pyenv":
		return "",
			fmt.Sprintf("pyenv uninstall %s", version),
			true
	case "fnm":
		return "",
			fmt.Sprintf("fnm uninstall %s", version),
			true
	case "rbenv":
		return "",
			fmt.Sprintf("rbenv uninstall %s", version),
			true
	case "system":
		// OS-specific instructions
		instructions := getSystemUninstallInstructions(runtimeDisplayName, dv.Path)
		return instructions, "", false
	default:
		// Unknown source - provide generic instructions
		return fmt.Sprintf("Manually remove the installation directory:\n  %s", dv.Path), "", false
	}
}

// getSystemUninstallInstructions provides OS-specific uninstall instructions for system packages
func getSystemUninstallInstructions(runtimeDisplayName string, path string) string {
	switch runtime.GOOS {
	case "windows":
		return "To uninstall:\n" +
			"  1. Open Settings → Apps → Installed apps\n" +
			"  2. Search for " + runtimeDisplayName + "\n" +
			"  3. Click Uninstall\n" +
			"  Or use PowerShell to find the uninstaller"
	case "darwin":
		return "To uninstall:\n" +
			"  If installed via Homebrew: brew uninstall " + strings.ToLower(runtimeDisplayName) + "\n" +
			"  If installed via package: check /Applications or use the installer's uninstaller\n" +
			"  Manual removal: sudo rm -rf " + path
	case "linux":
		return "To uninstall:\n" +
			"  If installed via apt: sudo apt remove " + strings.ToLower(runtimeDisplayName) + "\n" +
			"  If installed via yum: sudo yum remove " + strings.ToLower(runtimeDisplayName) + "\n" +
			"  If installed via dnf: sudo dnf remove " + strings.ToLower(runtimeDisplayName) + "\n" +
			"  Manual removal: sudo rm -rf " + path
	default:
		return "Manually remove the installation directory:\n  " + path
	}
}

// executeUninstallCommand executes the uninstall command for automated cleanup
func executeUninstallCommand(command string) error {
	// Parse the command into parts
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	// Execute the command
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
