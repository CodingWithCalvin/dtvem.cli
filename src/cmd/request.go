package cmd

import (
	"fmt"
	"net/url"
	"os/exec"
	goruntime "runtime"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/manifest"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
	"github.com/spf13/cobra"
)

const (
	buildRequestURL = "https://github.com/CodingWithCalvin/dtvem.cli/issues/new"
)

var requestCmd = &cobra.Command{
	Use:   "request <runtime> <version>",
	Short: "Request a build for an unavailable version",
	Long: `Request a pre-built binary for a version that is not currently available.

This command opens your browser to create a GitHub issue requesting a build
for the specified runtime and version on your current platform.

Example:
  dtvem request python 3.6.15
  dtvem request ruby 2.7.8`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		runtimeName := args[0]
		version := args[1]

		// Verify the runtime exists
		provider, err := runtime.Get(runtimeName)
		if err != nil {
			ui.Error("Unknown runtime: %s", runtimeName)
			ui.Info("Available runtimes: node, python, ruby")
			return
		}

		// Get current platform
		platform := manifest.CurrentPlatform()
		displayName := provider.DisplayName()

		// Build the issue URL with pre-filled fields
		issueURL := buildIssueURL(runtimeName, version, platform)

		ui.Info("Opening browser to request build for %s %s on %s...", displayName, version, platform)
		fmt.Println()

		err = openBrowser(issueURL)
		if err != nil {
			ui.Warning("Could not open browser automatically")
			fmt.Println()
			ui.Info("Please visit this URL manually:")
			fmt.Println()
			fmt.Println("  " + issueURL)
		}
	},
}

func buildIssueURL(runtimeName, version, platform string) string {
	title := fmt.Sprintf("build(%s): %s %s", runtimeName, version, platform)
	labels := fmt.Sprintf("build-request,%s,%s", runtimeName, platform)

	body := fmt.Sprintf(`## Build Request

**Runtime:** %s
**Version:** %s
**Platform:** %s

### Description

Please provide a pre-built binary for this version and platform.

### Additional Context

<!-- Add any additional context about why you need this version -->
`, runtimeName, version, platform)

	params := url.Values{}
	params.Set("title", title)
	params.Set("labels", labels)
	params.Set("body", body)

	return buildRequestURL + "?" + params.Encode()
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		// Try xdg-open first, fall back to sensible-browser
		cmd = exec.Command("xdg-open", url)
	case "windows":
		// Use cmd.exe to run start command
		cmd = exec.Command("cmd", "/c", "start", "", strings.ReplaceAll(url, "&", "^&"))
	default:
		return fmt.Errorf("unsupported platform: %s", goruntime.GOOS)
	}

	return cmd.Start()
}

func init() {
	rootCmd.AddCommand(requestCmd)
}
