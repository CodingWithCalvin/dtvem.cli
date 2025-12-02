package cmd

import (
	"fmt"

	"github.com/dtvem/dtvem/src/internal/ui"
	"github.com/spf13/cobra"
)

// Version can be set at build time using ldflags
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the dtvem version",
	Long:  `Display the current version of dtvem.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("dtvem version %s\n", ui.HighlightVersion(Version))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
