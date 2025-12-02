package cmd

import (
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

var localCmd = &cobra.Command{
	Use:   "local <runtime> <version>",
	Short: "Set the local version of a runtime for the current directory",
	Long: `Set a runtime version for the current directory by creating a .dtvem/runtimes.json file.
This version will be used when working in this directory or its subdirectories.

Examples:
  dtvem local python 3.11.0
  dtvem local node 18.16.0`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]
		version := args[1]

		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("%v", err)
			ui.Info("Available runtimes: %v", runtime.List())
			return
		}

		setRuntimeVersion(runtimeName, version, "local", provider.SetLocalVersion)
	},
}

func init() {
	rootCmd.AddCommand(localCmd)
}
