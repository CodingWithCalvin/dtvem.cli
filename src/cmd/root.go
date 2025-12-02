// Package cmd implements the CLI commands for dtvem
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "dtvem",
	Short:   "Dev Tool Virtual Environment Manager",
	Version: Version,
	Long: `dtvem is a cross-platform virtual environment manager for developer tools.
It allows you to manage multiple versions of Python, Node.js, and other runtimes,
with first-class Windows support.

Available commands:
  install  - Install a specific version of a runtime
  list     - List installed versions of a runtime
  global   - Set the global default version of a runtime
  local    - Set the local version for the current directory
  current  - Show the currently active version(s)
  migrate  - Migrate existing runtime installations to dtvem
  reshim   - Regenerate shim binaries
  version  - Show the dtvem version
  help     - Show help for any command`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	// Global flags and configuration initialization will go here
}
