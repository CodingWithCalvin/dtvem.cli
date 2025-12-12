// Package cmd implements the CLI commands for dtvem
package cmd

import (
	"fmt"
	"os"

	"github.com/dtvem/dtvem/src/internal/tui"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "dtvem",
	Short: "Developer Tools Virtual Environment Manager",
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		ui.SetVerbose(verbose)
	},
}

func Execute() {
	// Check for --version or -v flag before Cobra parses
	for _, arg := range os.Args[1:] {
		if arg == "--version" || arg == "-v" {
			versionCmd.Run(versionCmd, []string{})
			return
		}
	}

	if err := rootCmd.Execute(); err != nil {
		// Error already printed by Cobra, just exit with error code
		os.Exit(1)
	}
}

func init() {
	// Hide the completion command until we implement it
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	// Add global verbose flag
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output for debugging")

	// Set custom usage and help functions with TUI table for commands
	rootCmd.SetUsageFunc(customUsage)
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		_ = customUsage(cmd)
	})
}

func customUsage(cmd *cobra.Command) error {
	const tableWidth = 95 // Consistent width for all tables

	// Print header box with title and description
	headerTable := tui.NewTable("")
	headerTable.SetTitle(cmd.Short)
	headerTable.HideHeader()
	headerTable.SetMinWidth(tableWidth)
	headerTable.AddRow("DTVEM is a cross-platform virtual environment manager for multiple developer tools,")
	headerTable.AddRow("written in Go, with first-class support for Windows, MacOS, and Linux - right out of the box.")

	fmt.Println(headerTable.Render())
	fmt.Println()

	// Build commands table
	table := tui.NewTable("Command", "Description")
	table.SetTitle("Available Commands")
	table.SetMinWidth(tableWidth)

	for _, c := range cmd.Commands() {
		// Skip hidden commands and completion
		if c.Hidden || c.Name() == "completion" {
			continue
		}
		table.AddRow(c.Name(), c.Short)
	}

	fmt.Println(table.Render())

	return nil
}
