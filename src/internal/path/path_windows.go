//go:build windows

package path

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
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

	// pathActionMove is used to indicate the shims directory needs to be moved to the beginning of PATH
	pathActionMove = "move"
)

// RuntimeConflict represents a system-installed runtime that may conflict with dtvem
type RuntimeConflict struct {
	Name string // Display name (e.g., "Node.js")
	Path string // Full path to the executable
}

// AddToPath adds the shims directory to the PATH on Windows.
// If userInstall is true, it modifies the User PATH (no admin required).
// If userInstall is false, it modifies the System PATH (requires admin).
func AddToPath(shimsDir string, skipConfirmation bool, userInstall bool) error {
	if userInstall {
		return addToUserPath(shimsDir, skipConfirmation)
	}
	return addToSystemPath(shimsDir, skipConfirmation)
}

// addToSystemPath adds the shims directory to the System PATH on Windows.
// This requires administrator privileges. If not elevated, it will prompt
// the user to re-run with elevation (unless skipConfirmation is true).
func addToSystemPath(shimsDir string, skipConfirmation bool) error {
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
		return promptForElevation(shimsDir, action, skipConfirmation)
	}

	// We have admin privileges - proceed with modification
	return modifySystemPath(shimsDir, action)
}

// addToUserPath adds the shims directory to the User PATH on Windows.
// This does not require administrator privileges.
func addToUserPath(shimsDir string, skipConfirmation bool) error {
	// Check for system runtime conflicts first
	conflicts := detectSystemRuntimeConflicts()
	if len(conflicts) > 0 {
		continueInstall, err := warnAboutSystemConflicts(conflicts, skipConfirmation)
		if err != nil {
			return err
		}
		if !continueInstall {
			return nil
		}
	}

	// Check current User PATH status
	needsUpdate, action, err := checkUserPath(shimsDir)
	if err != nil {
		return err
	}

	if !needsUpdate {
		ui.Success("%s is already at the beginning of your User PATH", shimsDir)
		return nil
	}

	// Modify User PATH
	return modifyUserPath(shimsDir, action)
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
		return true, pathActionMove, nil // Exists but not at beginning
	}
	return true, "add", nil // Not in PATH
}

// checkUserPath checks if the shims directory needs to be added/moved in User PATH
// Returns: needsUpdate, action ("add" or "move"), error
func checkUserPath(shimsDir string) (bool, string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE)
	if err != nil {
		return false, "", fmt.Errorf("failed to open User PATH registry key: %w", err)
	}
	defer func() { _ = key.Close() }()

	currentPath, _, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return false, "", fmt.Errorf("failed to read User PATH: %w", err)
	}

	// If PATH doesn't exist yet, we need to add it
	if errors.Is(err, registry.ErrNotExist) || currentPath == "" {
		return true, "add", nil
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
		return true, pathActionMove, nil // Exists but not at beginning
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

// promptForElevation prompts the user to re-run dtvem init with admin privileges.
// If skipConfirmation is true, it will automatically re-launch with elevation.
func promptForElevation(shimsDir, action string, skipConfirmation bool) error {
	if action == pathActionMove {
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

	if !skipConfirmation {
		fmt.Printf("\nRe-run with administrator privileges? [Y/n]: ")

		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response != "" && response != constants.ResponseY && response != constants.ResponseYes {
			ui.Warning("PATH not modified. You can run 'dtvem init' again later.")
			return nil
		}
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

	if action == pathActionMove {
		ui.Success("Moved %s to the beginning of your System PATH", shimsDir)
	} else {
		ui.Success("Added %s to your System PATH", shimsDir)
	}
	ui.Warning("Please restart your terminal for the changes to take effect")

	return nil
}

// modifyUserPath modifies the User PATH (no admin privileges required)
func modifyUserPath(shimsDir, action string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open User PATH registry key for writing: %w", err)
	}
	defer func() { _ = key.Close() }()

	currentPath, _, err := key.GetStringValue("Path")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("failed to read User PATH: %w", err)
	}

	// Parse and filter current PATH entries
	var filteredPaths []string
	if currentPath != "" {
		paths := strings.Split(currentPath, ";")
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
	}

	// Build new PATH with shimsDir at the beginning
	newPath := shimsDir
	if len(filteredPaths) > 0 {
		newPath += ";" + strings.Join(filteredPaths, ";")
	}

	// Write back to registry
	err = key.SetStringValue("Path", newPath)
	if err != nil {
		return fmt.Errorf("failed to update User PATH in registry: %w", err)
	}

	// Broadcast WM_SETTINGCHANGE to notify running processes
	broadcastSettingChange()

	if action == pathActionMove {
		ui.Success("Moved %s to the beginning of your User PATH", shimsDir)
	} else {
		ui.Success("Added %s to your User PATH", shimsDir)
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

// detectSystemRuntimeConflicts checks if system-installed runtimes exist in the System PATH
// that would take priority over dtvem shims when using User PATH installation.
// It excludes the dtvem shims directory to avoid false positives.
func detectSystemRuntimeConflicts() []RuntimeConflict {
	var conflicts []RuntimeConflict

	// Get System PATH only (excluding User PATH)
	systemPath := getSystemPathOnly()
	if systemPath == "" {
		return conflicts
	}

	// Get dtvem shims directory to exclude from conflict detection
	shimsDir := ShimsDir()

	// Runtimes to check for
	runtimeChecks := []struct {
		execName    string
		displayName string
	}{
		{"node", "Node.js"},
		{"python", "Python"},
		{"ruby", "Ruby"},
	}

	pathDirs := strings.Split(systemPath, ";")

	for _, runtime := range runtimeChecks {
		for _, dir := range pathDirs {
			dir = strings.TrimSpace(dir)
			if dir == "" {
				continue
			}

			// Skip dtvem shims directory (case-insensitive on Windows)
			if strings.EqualFold(dir, shimsDir) {
				continue
			}

			// Check for .exe extension on Windows
			execPath := filepath.Join(dir, runtime.execName+".exe")
			if info, err := os.Stat(execPath); err == nil && !info.IsDir() {
				conflicts = append(conflicts, RuntimeConflict{
					Name: runtime.displayName,
					Path: execPath,
				})
				break // Found this runtime, move to next
			}
		}
	}

	return conflicts
}

// getSystemPathOnly reads the System PATH from registry (excludes User PATH)
func getSystemPathOnly() string {
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer func() { _ = key.Close() }()

	systemPath, _, err := key.GetStringValue("Path")
	if err != nil {
		return ""
	}

	return systemPath
}

// warnAboutSystemConflicts displays a warning about system-installed runtimes
// and prompts the user to continue or abort.
// Returns: (continueInstall, error)
func warnAboutSystemConflicts(conflicts []RuntimeConflict, skipConfirmation bool) (bool, error) {
	ui.Warning("System-installed runtimes detected that will take priority over dtvem:")
	for _, conflict := range conflicts {
		ui.Info("  - %s: %s", conflict.Name, ui.Highlight(conflict.Path))
	}

	ui.Info("")
	ui.Info("On Windows, System PATH is evaluated before User PATH.")
	ui.Info("These system runtimes will be used instead of dtvem-managed versions.")
	ui.Info("")
	ui.Info("Options:")
	ui.Info("  1. Uninstall the system runtimes to use dtvem-managed versions")
	ui.Info("  2. Run 'dtvem init' as administrator for system-level PATH (recommended)")
	ui.Info("  3. Continue with user install (system runtimes will take priority)")

	if skipConfirmation {
		// With -y flag, continue anyway but still show the warning
		ui.Info("")
		ui.Warning("Continuing with user install (--yes flag specified)")
		return true, nil
	}

	fmt.Printf("\nContinue with user install? [y/N]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	if response == constants.ResponseY || response == constants.ResponseYes {
		return true, nil
	}

	ui.Info("User install cancelled. Run 'dtvem init' without --user for system-level PATH.")
	return false, nil
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
