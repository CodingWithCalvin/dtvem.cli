package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"

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

		// If this is a secondary executable and the shim-map cache knows
		// which versions provide it, verify the active version is one of
		// them. This gives the user an informed "available in: X" message
		// instead of a generic "not found" when they're on a version that
		// doesn't include the command.
		if commandName != runtimeName {
			if entry, ok := shim.Lookup(commandName); ok && len(entry.Versions) > 0 {
				if !versionInList(version, entry.Versions) {
					reportNotAvailableInVersion(commandName, runtimeName, provider.DisplayName(), version, entry.Versions)
					return
				}
			}
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

// mapCommandToRuntime maps a command name to its runtime. It first consults
// the shim-map cache (which records dynamically-installed packages such as
// uv, tsc, black) and falls back to the registered providers' core shim
// lists when the cache has no entry.
func mapCommandToRuntime(commandName string) string {
	if runtimeName, ok := shim.LookupRuntime(commandName); ok {
		return runtimeName
	}

	for _, rt := range runtime.List() {
		for _, shimName := range shim.RuntimeShims(rt) {
			if shimName == commandName {
				return rt
			}
		}
	}

	return ""
}

// versionInList reports whether version is in the providing-versions list.
func versionInList(version string, providingVersions []string) bool {
	for _, v := range providingVersions {
		if v == version {
			return true
		}
	}
	return false
}

// reportNotAvailableInVersion prints the user-facing "not available in this
// runtime version" error for `dtvem which`, including the list of versions
// that DO provide the executable so the user can switch to one.
func reportNotAvailableInVersion(commandName, runtimeName, displayName, activeVersion string, providingVersions []string) {
	ui.Error("'%s' is not available in %s %s", commandName, displayName, activeVersion)

	labeled := make([]string, len(providingVersions))
	for i, v := range providingVersions {
		labeled[i] = fmt.Sprintf("%s %s", displayName, v)
	}
	ui.Info("Available in: %s", strings.Join(labeled, ", "))

	if len(providingVersions) == 1 {
		ui.Info("Switch with: dtvem global %s %s", runtimeName, providingVersions[0])
	} else {
		ui.Info("Switch with 'dtvem global %s <version>' or set a local version.", runtimeName)
	}
}

func init() {
	rootCmd.AddCommand(whichCmd)
}
