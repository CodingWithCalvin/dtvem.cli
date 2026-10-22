package cmd

import (
	"errors"
	"fmt"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/tui"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/spf13/cobra"
)

var (
	currentYes       bool
	currentNoInstall bool
)

// runtimeStatus holds the status of a configured runtime
type runtimeStatus struct {
	provider  runtime.Provider
	version   string
	source    string // file that set the version
	installed bool
}

var currentCmd = &cobra.Command{
	Use:   "current [runtime]",
	Short: "Show the currently active version(s)",
	Long: `Show the currently active version for a specific runtime or all runtimes.
The active version is determined by checking local settings first, then global settings.
Local settings are read from .dtvem/runtimes.json and, for Node.js, from
.node-version and .nvmrc, in the current directory or the nearest parent that
has one. The Source column shows which file set each version.

Examples:
  dtvem current           # Show all active versions
  dtvem current python    # Show active Python version
  dtvem current node      # Show active Node.js version`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			showAllVersions(currentYes, currentNoInstall)
		} else {
			showSingleVersion(args[0], currentYes, currentNoInstall)
		}
	},
}

// showAllVersions displays all configured runtimes and prompts to install missing ones.
// If noInstall is true, install prompts are skipped entirely.
// If yes is true, install prompts are auto-accepted.
func showAllVersions(yes, noInstall bool) {
	providers := runtime.GetAll()

	if len(providers) == 0 {
		ui.Info("No runtime providers registered")
		return
	}

	// Collect status for all configured runtimes
	var configured []runtimeStatus
	failed := false
	for _, provider := range providers {
		res, err := config.Resolve(provider.Name())
		if errors.Is(err, config.ErrNoVersion) {
			// Not configured - skip it
			continue
		}
		if err != nil {
			ui.Error("%s: %v", provider.DisplayName(), err)
			failed = true
			continue
		}
		installed, _ := provider.IsInstalled(res.Version)
		configured = append(configured, runtimeStatus{
			provider:  provider,
			version:   res.Version,
			source:    res.Source,
			installed: installed,
		})
	}

	if len(configured) == 0 {
		if !failed {
			ui.Info("No runtimes configured")
		}
		return
	}

	// Display all configured versions
	table := tui.NewTable("Runtime", "Version", "Status", "Source")
	table.SetTitle("Active Versions")
	var missing []runtimeStatus

	for _, rs := range configured {
		if rs.installed {
			table.AddActiveRow(rs.provider.DisplayName(), rs.version, tui.CheckMark+" installed", rs.source)
		} else {
			table.AddRow(rs.provider.DisplayName(), rs.version, tui.CrossMark+" not installed", rs.source)
			missing = append(missing, rs)
		}
	}

	fmt.Println(table.Render())

	// Skip install prompts if --no-install flag is set
	if noInstall {
		return
	}

	// Prompt to install missing versions
	if len(missing) > 0 {
		fmt.Println()
		shouldInstall := yes || ui.PromptInstallMissing(missing)
		if shouldInstall {
			for _, rs := range missing {
				installVersion(rs.provider, rs.version)
			}
		}
	}
}

// showSingleVersion displays a single runtime version and prompts to install if missing.
// If noInstall is true, install prompts are skipped entirely.
// If yes is true, install prompts are auto-accepted.
func showSingleVersion(runtimeName string, yes, noInstall bool) {
	provider, err := runtime.Get(runtimeName)
	if err != nil {
		ui.Error("%v", err)
		ui.Info("Available runtimes: %v", runtime.List())
		return
	}

	res, err := config.Resolve(provider.Name())
	if err != nil {
		ui.Error("%v", err)
		return
	}
	version := res.Version

	installed, _ := provider.IsInstalled(version)

	table := tui.NewTable("Runtime", "Version", "Status", "Source")
	if installed {
		table.AddActiveRow(provider.DisplayName(), version, tui.CheckMark+" installed", res.Source)
		fmt.Println(table.Render())
		return
	}

	// Not installed - show with warning and prompt
	table.AddRow(provider.DisplayName(), version, tui.CrossMark+" not installed", res.Source)
	fmt.Println(table.Render())

	// Skip install prompts if --no-install flag is set
	if noInstall {
		return
	}

	fmt.Println()
	shouldInstall := yes || ui.PromptInstall(provider.DisplayName(), version)
	if shouldInstall {
		installVersion(provider, version)
	}
}

// installVersion installs a configured version that is not installed yet.
// A version file can ask for a partial version ("22"), which is resolved to
// the latest available match first.
func installVersion(provider runtime.Provider, version string) {
	resolved, err := resolveVersionForProvider(provider, version)
	if err != nil {
		ui.Error("Failed to install %s %s: %v", provider.DisplayName(), version, err)
		return
	}

	ui.Info("Installing %s %s...", provider.DisplayName(), resolved)
	if err := provider.Install(resolved); err != nil {
		ui.Error("Failed to install %s %s: %v", provider.DisplayName(), resolved, err)
		return
	}
	ui.Success("%s %s installed successfully", provider.DisplayName(), resolved)
}

func init() {
	currentCmd.Flags().BoolVarP(&currentYes, "yes", "y", false, "Automatically install missing versions without prompting")
	currentCmd.Flags().BoolVarP(&currentNoInstall, "no-install", "n", false, "Skip install prompts entirely")
	rootCmd.AddCommand(currentCmd)
}
