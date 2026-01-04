package cmd

import (
	"fmt"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/tui"
	"github.com/spf13/cobra"
)

// Version can be set at build time using ldflags
var Version = "dev"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show the dtvem version",
	Long:  `Display the current version of dtvem.`,
	Run: func(cmd *cobra.Command, args []string) {
		content := fmt.Sprintf("dtvem %s", tui.RenderVersion(Version))
		fmt.Println(tui.RenderInfoBox(content))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
