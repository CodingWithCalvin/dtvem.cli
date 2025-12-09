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
	"golang.org/x/sys/windows"
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

// AddToPath adds the shims directory to the System PATH on Windows.
// This requires administrator privileges. If not elevated, it will prompt
// the user to re-run with elevation.
func AddToPath(shimsDir string) error {
	// Check current System PATH status
	needsUpdate, action, err := checkSystemPath(shimsDir)
	if err != nil {
		return err
	}

	if !needsUpdate {
		ui.Success("%s is already at the beginning of your System PATH", shimsDir)
		return nil
	}

	// Check if we have admin privileges
	if !isAdmin() {
		return promptForElevation(shimsDir, action)
	}

	// We have admin privileges - proceed with modification
	return modifySystemPath(shimsDir, action)
}

// checkSystemPath checks if the shims directory needs to be added/moved in System PATH
// Returns: needsUpdate, action ("add" or "move"), error
func checkSystemPath(shimsDir string) (bool, string, error) {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
	if err != nil {
		return false, "", fmt.Errorf("failed to open System PATH registry key: %w", err)
	}
	defer func() { _ = key.Close() }()

	currentPath, _, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return false, "", fmt.Errorf("failed to read System PATH: %w", err)
	}

	paths := strings.Split(currentPath, ";")
	foundAt := -1

	for i, p := range paths {
		trimmed := strings.TrimSpace(p)
		if strings.EqualFold(trimmed, shimsDir) {
			foundAt = i
			break
		}
	}

	if foundAt == 0 {
		return false, "", nil // Already at beginning
	} else if foundAt > 0 {
		return true, "move", nil // Exists but not at beginning
	}
	return true, "add", nil // Not in PATH
}

// isAdmin checks if the current process has administrator privileges
func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false
	}
	return true
}

// promptForElevation prompts the user to re-run dtvem init with admin privileges
func promptForElevation(shimsDir, action string) error {
	if action == "move" {
		ui.Header("PATH Fix Required (Administrator)")
		ui.Warning("%s is in your System PATH but not at the beginning", shimsDir)
		ui.Info("It needs to be first to take priority over other installations")
	} else {
		ui.Header("PATH Setup Required (Administrator)")
		ui.Info("dtvem needs to add the shims directory to your System PATH")
		ui.Info("Directory: %s", ui.Highlight(shimsDir))
	}

	ui.Info("")
	ui.Info("On Windows, System PATH takes priority over User PATH.")
	ui.Info("Modifying System PATH requires administrator privileges.")

	fmt.Printf("\nRe-run with administrator privileges? [Y/n]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "" && response != constants.ResponseY && response != constants.ResponseYes {
		ui.Warning("PATH not modified. You can run 'dtvem init' again later.")
		return nil
	}

	// Re-launch with elevation
	return relaunchElevated()
}

// relaunchElevated re-launches the current executable with administrator privileges
func relaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Use ShellExecute with "runas" verb to request elevation
	verb := windows.StringToUTF16Ptr("runas")
	exePath := windows.StringToUTF16Ptr(exe)
	args := windows.StringToUTF16Ptr("init")
	dir := windows.StringToUTF16Ptr(cwd)

	err = windows.ShellExecute(0, verb, exePath, args, dir, windows.SW_SHOWNORMAL)
	if err != nil {
		return fmt.Errorf("failed to elevate: %w", err)
	}

	ui.Info("Elevated process launched. Please complete the setup in the new window.")
	return nil
}

// modifySystemPath modifies the System PATH (requires admin privileges)
func modifySystemPath(shimsDir, action string) error {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open System PATH registry key for writing: %w", err)
	}
	defer func() { _ = key.Close() }()

	currentPath, _, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("failed to read System PATH: %w", err)
	}

	// Parse and filter current PATH entries
	paths := strings.Split(currentPath, ";")
	var filteredPaths []string

	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		// Skip if it's the shims dir (we'll prepend it)
		if strings.EqualFold(trimmed, shimsDir) {
			continue
		}
		filteredPaths = append(filteredPaths, trimmed)
	}

	// Build new PATH with shimsDir at the beginning
	newPath := shimsDir
	if len(filteredPaths) > 0 {
		newPath += ";" + strings.Join(filteredPaths, ";")
	}

	// Write back to registry
	err = key.SetStringValue("Path", newPath)
	if err != nil {
		return fmt.Errorf("failed to update System PATH in registry: %w", err)
	}

	// Broadcast WM_SETTINGCHANGE to notify running processes
	broadcastSettingChange()

	if action == "move" {
		ui.Success("Moved %s to the beginning of your System PATH", shimsDir)
	} else {
		ui.Success("Added %s to your System PATH", shimsDir)
	}
	ui.Warning("Please restart your terminal for the changes to take effect")

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
