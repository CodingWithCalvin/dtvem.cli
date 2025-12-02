//go:build windows

package path

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"

	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/ui"
	"golang.org/x/sys/windows/registry"
)

var (
	moduser32              = syscall.NewLazyDLL("user32.dll")
	procSendMessageTimeout = moduser32.NewProc("SendMessageTimeoutW")
)

const (
	HWND_BROADCAST   = 0xffff
	WM_SETTINGCHANGE = 0x001A
	SMTO_ABORTIFHUNG = 0x0002
)

// AddToPath adds the shims directory to the user's PATH on Windows
func AddToPath(shimsDir string) error {
	// Check if already in PATH
	if IsInPath(shimsDir) {
		ui.Info("%s is already in your PATH", shimsDir)
		return nil
	}

	// Prompt user for confirmation
	ui.Header("PATH Setup Required")
	ui.Info("dtvem needs to add the shims directory to your PATH")
	ui.Info("Directory: %s", ui.Highlight(shimsDir))
	ui.Info("This will modify your user PATH environment variable")
	fmt.Printf("\nProceed? [Y/n]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "" && response != constants.ResponseY && response != constants.ResponseYes {
		ui.Warning("PATH not modified. You can add it manually later by running: dtvem init")
		return nil
	}

	// Get current user PATH from registry
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open registry key: %w", err)
	}
	defer func() { _ = key.Close() }()

	currentPath, _, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("failed to read current PATH: %w", err)
	}

	// Check if already present (double-check)
	paths := strings.Split(currentPath, ";")
	for _, p := range paths {
		if strings.EqualFold(strings.TrimSpace(p), shimsDir) {
			ui.Info("%s is already in your registry PATH", shimsDir)
			return nil
		}
	}

	// Prepend the shims directory to the BEGINNING for priority
	newPath := shimsDir
	if currentPath != "" {
		newPath += ";" + currentPath
	}

	// Write back to registry
	err = key.SetStringValue("Path", newPath)
	if err != nil {
		return fmt.Errorf("failed to update PATH in registry: %w", err)
	}

	// Broadcast WM_SETTINGCHANGE to notify running processes
	broadcastSettingChange()

	ui.Success("Added %s to your PATH", shimsDir)
	ui.Warning("Please restart your terminal for the changes to take effect")
	ui.Info("You can verify by running: echo %%PATH%%")

	return nil
}

// broadcastSettingChange broadcasts WM_SETTINGCHANGE to notify the system of environment changes
func broadcastSettingChange() {
	env := syscall.StringToUTF16Ptr("Environment")
	_, _, _ = procSendMessageTimeout.Call(
		uintptr(HWND_BROADCAST),
		uintptr(WM_SETTINGCHANGE),
		0,
		uintptr(unsafe.Pointer(env)),
		uintptr(SMTO_ABORTIFHUNG),
		5000, // 5 second timeout
		0,
	)
}

// DetectShell returns "powershell" or "cmd" on Windows (not actually used, but for consistency)
func DetectShell() string {
	// Check if running in PowerShell
	if os.Getenv("PSModulePath") != "" {
		return "powershell"
	}
	return "cmd"
}

// GetShellConfigFile returns empty string on Windows (no shell config files)
func GetShellConfigFile(shell string) string {
	// Windows doesn't use shell config files for PATH
	return ""
}

// IsSetxAvailable checks if setx command is available (fallback method)
func IsSetxAvailable() bool {
	_, err := exec.LookPath("setx")
	return err == nil
}
