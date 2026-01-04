//go:build !windows

package path

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
)

// DetectShell returns the user's shell name (bash, zsh, fish, etc.)
func DetectShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		return "unknown"
	}

	// Extract just the shell name from the path
	return filepath.Base(shell)
}

// GetShellConfigFile returns the config file path for the given shell
func GetShellConfigFile(shell string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	switch shell {
	case "bash":
		// Prefer .bashrc if it exists, otherwise .bash_profile
		bashrc := filepath.Join(home, ".bashrc")
		if _, err := os.Stat(bashrc); err == nil {
			return bashrc
		}
		return filepath.Join(home, ".bash_profile")

	case "zsh":
		return filepath.Join(home, ".zshrc")

	case constants.ShellFish:
		return filepath.Join(home, ".config", "fish", "config.fish")

	default:
		// Try .profile as a fallback
		return filepath.Join(home, ".profile")
	}
}

// AddToPath adds the shims directory to the user's PATH by modifying their shell config.
// If skipConfirmation is true, the function will proceed without prompting the user.
func AddToPath(shimsDir string, skipConfirmation bool) error {
	shell := DetectShell()
	if shell == "unknown" {
		return fmt.Errorf("could not detect shell - please add %s to your PATH manually", shimsDir)
	}

	configFile := GetShellConfigFile(shell)
	if configFile == "" {
		return fmt.Errorf("could not determine config file for shell %s", shell)
	}

	// Check if the directory is already in PATH
	if IsInPath(shimsDir) {
		ui.Info("%s is already in your PATH", shimsDir)
		return nil
	}

	// Check if the config file already contains the PATH modification
	if containsPathModification(configFile, shimsDir) {
		ui.Warning("PATH modification already exists in %s, but not active in current shell", configFile)
		ui.Info("Please restart your terminal or run: source %s", configFile)
		return nil
	}

	// Prepare the export statement
	exportLine := ""
	if shell == constants.ShellFish {
		exportLine = fmt.Sprintf("\n# Added by dtvem\nset -gx PATH \"%s\" $PATH\n", shimsDir)
	} else {
		exportLine = fmt.Sprintf("\n# Added by dtvem\nexport PATH=\"%s:$PATH\"\n", shimsDir)
	}

	// Prompt user for confirmation (unless skipConfirmation is true)
	if !skipConfirmation {
		ui.Header("PATH Setup Required")
		ui.Info("dtvem needs to add the shims directory to your PATH")
		ui.Info("Shell: %s", ui.Highlight(shell))
		ui.Info("Config file: %s", ui.Highlight(configFile))
		ui.Info("Will append: %s", ui.Highlight(strings.TrimSpace(exportLine)))
		fmt.Printf("\nProceed? [Y/n]: ")

		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "" && response != constants.ResponseY && response != constants.ResponseYes {
			ui.Warning("PATH not modified. Please add this manually to your %s:", configFile)
			ui.Info("%s", strings.TrimSpace(exportLine))
			return nil
		}
	}

	// Ensure the directory exists for fish config
	if shell == constants.ShellFish {
		configDir := filepath.Dir(configFile)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
	}

	// Append to the config file
	f, err := os.OpenFile(configFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(exportLine); err != nil {
		return fmt.Errorf("failed to write to config file: %w", err)
	}

	ui.Success("Added %s to PATH in %s", shimsDir, configFile)
	ui.Warning("Please restart your terminal or run: source %s", configFile)

	return nil
}

// containsPathModification checks if the config file already has dtvem PATH modification
func containsPathModification(configFile, shimsDir string) bool {
	f, err := os.Open(configFile)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		// Check if line mentions both dtvem/shims and PATH
		if strings.Contains(line, shimsDir) && (strings.Contains(line, "PATH") || strings.Contains(line, "path")) {
			return true
		}
	}

	return false
}
