// Package main implements the shim executable that intercepts runtime commands
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dtvem/dtvem/src/internal/config"
	"github.com/dtvem/dtvem/src/internal/constants"
	"github.com/dtvem/dtvem/src/internal/path"
	"github.com/dtvem/dtvem/src/internal/runtime"
	"github.com/dtvem/dtvem/src/internal/shim"
	"github.com/dtvem/dtvem/src/internal/ui"

	// Import runtime providers to register them
	_ "github.com/dtvem/dtvem/src/runtimes/node"
	_ "github.com/dtvem/dtvem/src/runtimes/python"
	_ "github.com/dtvem/dtvem/src/runtimes/ruby"
)

func main() {
	// Check for DTVEM_VERBOSE environment variable
	ui.CheckVerboseEnv()

	if err := runShim(); err != nil {
		fmt.Fprintf(os.Stderr, "dtvem shim error: %v\n", err)
		os.Exit(1)
	}
}

func runShim() error {
	// Get the name of this shim (e.g., "python", "node", "npm")
	shimName := getShimName()
	ui.Debug("Shim invoked: %s", shimName)
	ui.Debug("Arguments: %v", os.Args[1:])

	// Determine which runtime this shim belongs to
	runtimeName := mapShimToRuntime(shimName)
	ui.Debug("Mapped to runtime: %s", runtimeName)

	// Get the runtime provider (using ShimProvider interface for minimal dependencies)
	provider, err := runtime.GetShimProvider(runtimeName)
	if err != nil {
		ui.Debug("Provider lookup failed: %v", err)
		return fmt.Errorf("runtime provider not found: %w", err)
	}

	// Resolve which version to use
	version, err := config.ResolveVersion(runtimeName)
	if err != nil {
		ui.Debug("Version resolution failed: %v", err)
		// No dtvem version configured - try to fallback to system PATH
		return handleNoConfiguredVersion(shimName, runtimeName, provider)
	}
	ui.Debug("Resolved version: %s", version)

	// Check if the version is installed
	installed, err := provider.IsInstalled(version)
	if err != nil {
		return fmt.Errorf("could not check if %s %s is installed: %w", runtimeName, version, err)
	}

	if !installed {
		ui.Debug("Version %s is not installed", version)
		ui.Error("%s %s is configured but not installed", provider.DisplayName(), version)
		ui.Info("To install, run: dtvem install %s %s", runtimeName, version)
		return fmt.Errorf("version not installed")
	}

	// Get the path to the actual executable
	execPath, err := provider.ExecutablePath(version)
	if err != nil {
		return fmt.Errorf("could not find %s %s executable: %w", runtimeName, version, err)
	}
	ui.Debug("Base executable path: %s", execPath)

	// If the shim name differs from the base runtime name,
	// we might need to adjust the executable path
	// (e.g., python3 -> python3, pip -> pip, npm -> npm)
	execPath = adjustExecutablePath(execPath, shimName, runtimeName)
	ui.Debug("Final executable path: %s", execPath)

	// Get provider-specific environment variables (e.g., LD_LIBRARY_PATH for Ruby)
	providerEnv, err := provider.GetEnvironment(version)
	if err != nil {
		ui.Debug("Failed to get provider environment: %v", err)
		providerEnv = map[string]string{}
	}
	for k, v := range providerEnv {
		ui.Debug("Provider env: %s=%s", k, v)
	}

	// Check if this command should trigger a reshim after execution
	needsReshim := provider.ShouldReshimAfter(shimName, os.Args[1:])

	// Execute the actual binary
	if needsReshim {
		// Need to run code after execution, so use exec.Command
		exitCode := executeCommandWithWait(execPath, os.Args[1:], providerEnv)

		// If command succeeded, prompt for reshim
		if exitCode == 0 {
			promptReshim()
		}

		os.Exit(exitCode)
	} else {
		// Normal execution - use syscall.Exec on Unix for efficiency
		if err := executeCommand(execPath, os.Args[1:], providerEnv); err != nil {
			return fmt.Errorf("failed to execute %s: %w", execPath, err)
		}
	}

	return nil
}

// handleNoConfiguredVersion handles the case when no dtvem version is configured
// It attempts to fallback to system PATH or prompts for installation
func handleNoConfiguredVersion(shimName, runtimeName string, provider runtime.ShimProvider) error {
	// Try to find the executable deeper in PATH (system installation)
	systemPath := path.LookPathExcludingShims(shimName)

	if systemPath != "" {
		// Found system installation - use it
		ui.Info("No dtvem version configured for %s", provider.DisplayName())
		ui.Info("Using system installation: %s", systemPath)
		ui.Info("To manage with dtvem, run: dtvem install %s <version>", runtimeName)
		ui.Info("Or see available versions: dtvem list-all %s", runtimeName)
		fmt.Fprintln(os.Stderr) // Empty line for spacing

		// Execute the system version (no provider env needed for system installations)
		if err := executeCommand(systemPath, os.Args[1:], nil); err != nil {
			return fmt.Errorf("failed to execute system %s: %w", shimName, err)
		}
		return nil
	}

	// No system installation found either - prompt for installation
	ui.Warning("No dtvem version configured for %s", provider.DisplayName())
	ui.Warning("No system installation found in PATH")
	ui.Info("")
	ui.Info("To install with dtvem:")
	ui.Info("  1. See available versions: dtvem list-all %s", runtimeName)
	ui.Info("  2. Install a version: dtvem install %s <version>", runtimeName)
	ui.Info("  3. Set it globally: dtvem global %s <version>", runtimeName)
	ui.Info("")
	ui.Info("Or configure a local version in your project:")
	ui.Info("  dtvem local %s <version>", runtimeName)

	return fmt.Errorf("no version configured")
}

// getShimName returns the name of this shim binary
func getShimName() string {
	shimPath := os.Args[0]
	shimName := filepath.Base(shimPath)

	// Remove .exe extension on Windows
	shimName = strings.TrimSuffix(shimName, ".exe")

	return shimName
}

// mapShimToRuntime maps a shim name to its runtime
// For example: python3 -> python, pip -> python, npm -> node, tsc -> node
// First checks the shim map cache (generated by reshim), then falls back
// to querying registered providers.
func mapShimToRuntime(shimName string) string {
	// First, try the shim map cache for O(1) lookup
	// This handles both core shims and dynamically installed packages (tsc, eslint, black, etc.)
	if runtimeName, ok := shim.LookupRuntime(shimName); ok {
		return runtimeName
	}

	// Fall back to provider-based lookup if cache is missing or doesn't have the shim
	// Get all registered providers (using ShimProvider interface)
	providers := runtime.GetAllShimProviders()

	// Check each provider's shims for an exact match first
	for _, provider := range providers {
		for _, s := range provider.Shims() {
			if s == shimName {
				return provider.Name()
			}
		}
	}

	// Check for prefix match (e.g., python3 -> python)
	for _, provider := range providers {
		for _, s := range provider.Shims() {
			if strings.HasPrefix(shimName, s) {
				return provider.Name()
			}
		}
	}

	// Default: use shim name as runtime name
	return shimName
}

// adjustExecutablePath adjusts the executable path based on the shim name
// For example, if shim is "pip" but base executable is "python",
// we need to find "pip" in the same directory or Scripts subdirectory
func adjustExecutablePath(execPath, shimName, runtimeName string) string {
	// If shim name matches runtime name, use the path as-is
	if shimName == runtimeName {
		return execPath
	}

	// Otherwise, try to find the related executable
	// For example: if execPath is /path/to/python and shimName is pip,
	// look for /path/to/pip
	dir := filepath.Dir(execPath)

	// Directories to search (in order)
	searchDirs := []string{
		dir,                                 // Same directory as runtime executable
		filepath.Join(dir, "Scripts"),       // Python Scripts directory (Windows)
		filepath.Join(dir, "..", "Scripts"), // Alternative Python Scripts location
	}

	// On Windows, try multiple extensions
	if os.PathSeparator == '\\' {
		for _, searchDir := range searchDirs {
			newExec := filepath.Join(searchDir, shimName)

			// Try .cmd first (npm, npx use .cmd on Windows)
			if _, err := os.Stat(newExec + ".cmd"); err == nil {
				return newExec + ".cmd"
			}
			// Try .exe
			if _, err := os.Stat(newExec + ".exe"); err == nil {
				return newExec + ".exe"
			}
		}
	} else {
		// On Unix, check if the file exists as-is
		for _, searchDir := range searchDirs {
			newExec := filepath.Join(searchDir, shimName)
			if _, err := os.Stat(newExec); err == nil {
				return newExec
			}
		}
	}

	// If not found, return original path
	// The runtime provider should have returned the correct path
	return execPath
}

// executeCommand executes a command with the given arguments and provider environment
func executeCommand(execPath string, args []string, providerEnv map[string]string) error {
	// Build full args (executable name + arguments)
	fullArgs := append([]string{execPath}, args...)

	// Get current environment and apply provider overrides
	env := mergeEnvironment(os.Environ(), providerEnv)

	// On Unix systems, use Exec to replace the current process
	// On Windows, Exec is not available, so we use StartProcess
	if err := syscall.Exec(execPath, fullArgs, env); err != nil {
		// If Exec fails (e.g., on Windows), fall back to starting a new process
		cmd := &exec.Cmd{
			Path:   execPath,
			Args:   fullArgs,
			Env:    env,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
		if err := cmd.Run(); err != nil {
			// Check if this is an exit error (command ran but returned non-zero)
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				// Command executed successfully but returned non-zero exit code
				// This is not a shim error - propagate the exit code
				os.Exit(exitErr.ExitCode())
			}
			// Other errors (couldn't start, etc.) are actual failures
			return err
		}
	}

	return nil
}

// executeCommandWithWait executes a command and waits for it to complete, returning the exit code
func executeCommandWithWait(execPath string, args []string, providerEnv map[string]string) int {
	// Build full args (executable name + arguments)
	fullArgs := append([]string{execPath}, args...)

	// Get current environment and apply provider overrides
	env := mergeEnvironment(os.Environ(), providerEnv)

	// Use exec.Command to run the command and wait for completion
	cmd := &exec.Cmd{
		Path:   execPath,
		Args:   fullArgs,
		Env:    env,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}

	if err := cmd.Run(); err != nil {
		// Check if this is an exit error
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode()
		}
		// Other errors (couldn't start, etc.) return 1
		return 1
	}

	return 0
}

// mergeEnvironment merges provider environment variables into the base environment.
// Provider variables are prepended to existing values (for PATH-like variables) or set directly.
func mergeEnvironment(baseEnv []string, providerEnv map[string]string) []string {
	if len(providerEnv) == 0 {
		return baseEnv
	}

	// Build a map of existing environment variables for easy lookup
	envMap := make(map[string]string)
	for _, e := range baseEnv {
		if idx := strings.Index(e, "="); idx != -1 {
			key := e[:idx]
			value := e[idx+1:]
			envMap[key] = value
		}
	}

	// Apply provider environment variables
	// For PATH-like variables (LD_LIBRARY_PATH, DYLD_LIBRARY_PATH), prepend the new value
	for key, value := range providerEnv {
		if existing, ok := envMap[key]; ok && existing != "" {
			// Prepend new value to existing (for PATH-like variables)
			envMap[key] = value + string(filepath.ListSeparator) + existing
		} else {
			envMap[key] = value
		}
	}

	// Convert back to slice format
	result := make([]string, 0, len(envMap))
	for key, value := range envMap {
		result = append(result, key+"="+value)
	}

	return result
}

// promptReshim prompts the user to run reshim after installing global packages
func promptReshim() {
	fmt.Fprintln(os.Stderr) // Empty line for spacing
	ui.Info("Global packages were installed/removed")
	fmt.Fprintf(os.Stderr, "Run 'dtvem reshim' to update shims? [Y/n]: ")

	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	// Default to "yes" if empty response
	if response == "" || response == constants.ResponseY || response == constants.ResponseYes {
		// Run reshim
		if err := runReshim(); err != nil {
			ui.Error("Failed to run reshim: %v", err)
			ui.Info("Please run manually: dtvem reshim")
		} else {
			ui.Success("Shims updated successfully")
		}
	} else {
		ui.Info("Remember to run 'dtvem reshim' when you want to use the new executables")
	}
}

// runReshim executes the reshim operation
func runReshim() error {
	// Find dtvem executable
	dtvemPath, err := findDtvemExecutable()
	if err != nil {
		return err
	}

	// Run: dtvem reshim
	cmd := exec.Command(dtvemPath, "reshim")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// findDtvemExecutable locates the dtvem executable
func findDtvemExecutable() (string, error) {
	// Get the directory where this shim is located
	shimPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not determine shim path: %w", err)
	}

	shimDir := filepath.Dir(shimPath)

	// dtvem should be in ~/.dtvem/bin
	// The shim is also in ~/.dtvem/bin (or ~/.dtvem/shims in older versions)
	// Look for dtvem in the bin directory
	dtvemName := "dtvem"
	if os.PathSeparator == '\\' {
		dtvemName = "dtvem.exe"
	}

	// Try same directory first
	dtvemPath := filepath.Join(shimDir, dtvemName)
	if _, err := os.Stat(dtvemPath); err == nil {
		return dtvemPath, nil
	}

	// Try ~/.dtvem/bin
	paths := config.DefaultPaths()
	binDir := filepath.Join(paths.Root, "bin")
	dtvemPath = filepath.Join(binDir, dtvemName)
	if _, err := os.Stat(dtvemPath); err == nil {
		return dtvemPath, nil
	}

	// Last resort: search PATH
	dtvemPath, err = exec.LookPath("dtvem")
	if err == nil {
		return dtvemPath, nil
	}

	return "", fmt.Errorf("could not find dtvem executable")
}
