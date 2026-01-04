package cmd

import (
	"fmt"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/tui"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
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
	ui.Debug("Listing installed versions for all runtimes")

	providers := runtime.GetAll()
	ui.Debug("Found %d registered providers", len(providers))

	if len(providers) == 0 {
		ui.Info("No runtime providers registered")
		return
	}

	hasAny := false
	for _, provider := range providers {
		ui.Debug("Checking provider: %s", provider.Name())
		versions, err := provider.ListInstalled()
		if err != nil {
			ui.Debug("Error listing versions for %s: %v", provider.Name(), err)
			ui.Error("  %s: %v", provider.DisplayName(), err)
			continue
		}
		ui.Debug("Found %d installed versions for %s", len(versions), provider.Name())

		if len(versions) == 0 {
			continue
		}

		hasAny = true
		runtimeName := provider.Name()
		globalVersion, _ := provider.GlobalVersion()
		localVersion, _ := config.LocalVersion(runtimeName)

		// Create table for this runtime with title
		table := tui.NewTable("Version", "Status")
		table.SetTitle(provider.DisplayName())

		for _, v := range versions {
			version := v.String()
			status := getVersionStatus(version, globalVersion, localVersion)
			isActive := isVersionActive(version, globalVersion, localVersion)

			if isActive {
				table.AddActiveRow(version, status)
			} else {
				table.AddRow(version, status)
			}
		}

		fmt.Println(table.Render())
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

	// Create table with title
	table := tui.NewTable("Version", "Status")
	table.SetTitle(provider.DisplayName())

	for _, v := range versions {
		version := v.String()
		status := getVersionStatus(version, globalVersion, localVersion)
		isActive := isVersionActive(version, globalVersion, localVersion)

		if isActive {
			table.AddActiveRow(version, status)
		} else {
			table.AddRow(version, status)
		}
	}

	fmt.Println(table.Render())
}

// getVersionStatus returns a status string for a version (global, local, or empty)
func getVersionStatus(version, globalVersion, localVersion string) string {
	isGlobal := version == globalVersion
	isLocal := version == localVersion

	var parts []string
	if isLocal {
		parts = append(parts, localIndicator+" local")
	}
	if isGlobal {
		parts = append(parts, globalIndicator+" global")
	}

	if len(parts) == 0 {
		return ""
	}

	status := ""
	for i, p := range parts {
		if i > 0 {
			status += ", "
		}
		status += p
	}
	return status
}

// isVersionActive returns true if this version is the currently active one
func isVersionActive(version, globalVersion, localVersion string) bool {
	isGlobal := version == globalVersion
	isLocal := version == localVersion
	return isLocal || (isGlobal && localVersion == "")
}

func init() {
	rootCmd.AddCommand(listCmd)
}
