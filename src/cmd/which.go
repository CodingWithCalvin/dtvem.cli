package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
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

		// Resolve secondary executables (pip, npm, uv, etc.) by searching
		// the runtime install. If the shim name matches the runtime name,
		// the runtime executable itself is the answer.
		execPath := baseExecPath
		if commandName != runtimeName {
			resolved, err := shim.FindSecondaryExecutable(baseExecPath, commandName)
			if err != nil {
				ui.Error("'%s' is not available in %s %s", commandName, provider.DisplayName(), version)
				ui.Info("This shim exists because another installed %s version provides it.", provider.DisplayName())
				ui.Info("Install '%s' for the active version, or switch to a version that has it.", commandName)
				return
			}
			execPath = resolved
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

func init() {
	rootCmd.AddCommand(whichCmd)
}
