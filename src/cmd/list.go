package cmd

import (
	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

// Version indicator emojis
const (
	globalIndicator = "🌐"
	localIndicator  = "📍"
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
		runtimeName := provider.Name()
		globalVersion, _ := provider.GlobalVersion()
		localVersion, _ := config.LocalVersion(runtimeName)

		ui.Printf("  %s:\n", ui.Highlight(provider.DisplayName()))
		for _, v := range versions {
			printVersionLine(v.String(), globalVersion, localVersion)
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
	localVersion, _ := config.LocalVersion(runtimeName)

	for _, v := range versions {
		printVersionLine(v.String(), globalVersion, localVersion)
	}
}

// printVersionLine prints a single version with appropriate indicators and colors
// Active version (local > global) is shown in green
// Indicators: 🌐 for global, 📍 for local
func printVersionLine(version, globalVersion, localVersion string) {
	isGlobal := version == globalVersion
	isLocal := version == localVersion

	// Determine if this is the active version (local takes priority over global)
	isActive := isLocal || (isGlobal && localVersion == "")

	// Build the indicator string
	var indicators string
	if isLocal {
		indicators += " " + localIndicator
	}
	if isGlobal {
		indicators += " " + globalIndicator
	}

	// Format and print
	if isActive {
		ui.Printf("    %s%s\n", ui.ActiveVersion(version), indicators)
	} else {
		ui.Printf("    %s%s\n", version, indicators)
	}
}

func init() {
	rootCmd.AddCommand(listCmd)
}
