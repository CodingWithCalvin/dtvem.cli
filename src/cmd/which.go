package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/shim"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var whichCmd = &cobra.Command{
	Use:   "which <command>",
	Short: "Show the path to a command",
	Long: `Display the full path to a command and which shim is being used.

This command shows:
  - The shim path that intercepts the command
  - The actual executable that will be invoked
  - The runtime and version being used

Examples:
  dtvem which python
  dtvem which node
  dtvem which npm`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		commandName := args[0]

		// Find which runtime this command belongs to
		runtimeName := mapCommandToRuntime(commandName)
		if runtimeName == "" {
			ui.Error("Unknown command: %s", commandName)
			ui.Info("This command is not managed by dtvem")
			return
		}

		// Get the provider for this runtime
		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("Runtime provider not found: %s", runtimeName)
			return
		}

		// Get the shim path
		paths := config.DefaultPaths()
		shimExt := ""
		if goruntime.GOOS == constants.OSWindows {
			shimExt = ".exe"
		}
		shimPath := filepath.Join(paths.Shims, commandName+shimExt)

		// Check if shim exists
		if _, err := os.Stat(shimPath); os.IsNotExist(err) {
			ui.Error("Shim not found: %s", commandName)
			ui.Info("Run 'dtvem reshim' to regenerate shims")
			return
		}

		// Get the current version
		version, err := provider.CurrentVersion()
		if err != nil {
			ui.Error("No version configured for %s", runtimeName)
			ui.Info("Set a version with: dtvem global %s <version>", runtimeName)
			return
		}

		// Get the base executable path
		baseExecPath, err := provider.ExecutablePath(version)
		if err != nil {
			ui.Error("Failed to get executable path: %v", err)
			return
		}

		// Adjust path for secondary executables (pip, npm, etc.)
		execPath := adjustExecutablePath(baseExecPath, commandName, runtimeName)

		// Check if the actual executable exists
		if _, err := os.Stat(execPath); os.IsNotExist(err) {
			ui.Error("Executable not found: %s", execPath)
			ui.Warning("Version %s may not be properly installed", version)
			return
		}

		// Display the information
		ui.Header("Command: %s", ui.Highlight(commandName))
		fmt.Println()
		ui.Info("Shim:       %s", shimPath)
		ui.Info("Executable: %s", execPath)
		ui.Info("Runtime:    %s", runtimeName)
		ui.Info("Version:    %s", ui.HighlightVersion(version))
	},
}

// mapCommandToRuntime maps a command name to its runtime
func mapCommandToRuntime(commandName string) string {
	// Get all registered runtimes
	runtimes := runtime.List()

	// Check each runtime's shims
	for _, rt := range runtimes {
		shims := shim.RuntimeShims(rt)
		for _, shimName := range shims {
			if shimName == commandName {
				return rt
			}
		}
	}

	return ""
}

// adjustExecutablePath adjusts the executable path based on the command name
// For example, if command is "pip" but base executable is "python",
// we need to find "pip" in the same directory or Scripts subdirectory
func adjustExecutablePath(execPath, commandName, runtimeName string) string {
	// If command name matches runtime name, use the path as-is
	if commandName == runtimeName {
		return execPath
	}

	// Otherwise, try to find the related executable
	dir := filepath.Dir(execPath)

	// Directories to search (in order)
	searchDirs := []string{
		dir,                                 // Same directory as runtime executable
		filepath.Join(dir, "Scripts"),       // Python Scripts directory (Windows)
		filepath.Join(dir, "..", "Scripts"), // Alternative Python Scripts location
	}

	// On Windows, try multiple extensions
	if goruntime.GOOS == "windows" {
		for _, searchDir := range searchDirs {
			newExec := filepath.Join(searchDir, commandName)

			// Try .cmd first (npm, npx use .cmd on Windows)
			if _, err := os.Stat(newExec + ".cmd"); err == nil {
				return newExec + ".cmd"
			}
			// Try .exe
			if _, err := os.Stat(newExec + ".exe"); err == nil {
				return newExec + ".exe"
			}
		}
	} else {
		// On Unix, check if the file exists as-is
		for _, searchDir := range searchDirs {
			newExec := filepath.Join(searchDir, commandName)
			if _, err := os.Stat(newExec); err == nil {
				return newExec
			}
		}
	}

	// If not found, return original path
	return execPath
}

func init() {
	rootCmd.AddCommand(whichCmd)
}
