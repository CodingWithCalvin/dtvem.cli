package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RuntimesConfig represents the flat structure of runtimes.json
// Format: {"python": "3.11.0", "node": "18.16.0"}
type RuntimesConfig map[string]string

// SchemaURL is the URL to the runtimes.json schema
const SchemaURL = "https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/schemas/runtimes.schema.json"

// ResolveVersion finds the version to use for a runtime
// Priority: local dtvem.config.json file (walking up directory tree) > global config
func ResolveVersion(runtimeName string) (string, error) {
	// First, try to find local version
	localVersion, err := findLocalVersion(runtimeName)
	if err == nil && localVersion != "" {
		return localVersion, nil
	}

	// Fall back to global version
	globalVersion, err := GlobalVersion(runtimeName)
	if err == nil && globalVersion != "" {
		return globalVersion, nil
	}

	return "", fmt.Errorf("no version configured for %s", runtimeName)
}

// findLocalVersion walks up the directory tree looking for .dtvem/runtimes.json file
// Stops at filesystem root
func findLocalVersion(runtimeName string) (string, error) {
	// Start from current working directory
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree
	for {
		configDir := filepath.Join(currentDir, LocalConfigDirName)
		versionFile := filepath.Join(configDir, RuntimesFileName)

		// Check if .dtvem/runtimes.json exists
		if _, err := os.Stat(versionFile); err == nil {
			version, err := readVersionFile(versionFile, runtimeName)
			if err == nil && version != "" {
				return version, nil
			}
		}

		// Move up one directory
		parent := filepath.Dir(currentDir)

		// Stop if we've reached the filesystem root
		if parent == currentDir {
			break
		}

		currentDir = parent
	}

	return "", fmt.Errorf("no local version file found")
}

// readVersionFile reads a JSON config file and extracts the version for a runtime
// Format: {"python": "3.11.0", "node": "18.16.0"}
func readVersionFile(filePath, runtimeName string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	var config RuntimesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return "", fmt.Errorf("failed to parse config file: %w", err)
	}

	version, ok := config[runtimeName]
	if !ok {
		return "", fmt.Errorf("runtime %s not found in config file", runtimeName)
	}

	return version, nil
}

// ReadAllRuntimes reads all runtime/version pairs from a config file
// Returns a RuntimesConfig map with all runtimes, or an error if file doesn't exist or is invalid
func ReadAllRuntimes(filePath string) (RuntimesConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var config RuntimesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// FindLocalRuntimesFile walks up the directory tree looking for .dtvem/runtimes.json
// Returns the path to the file if found, or an error
func FindLocalRuntimesFile() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	// Walk up the directory tree
	for {
		configDir := filepath.Join(currentDir, LocalConfigDirName)
		versionFile := filepath.Join(configDir, RuntimesFileName)

		// Check if .dtvem/runtimes.json exists
		if _, err := os.Stat(versionFile); err == nil {
			return versionFile, nil
		}

		// Move up one directory
		parent := filepath.Dir(currentDir)

		// Stop if we've reached the filesystem root
		if parent == currentDir {
			break
		}

		currentDir = parent
	}

	return "", fmt.Errorf("no .dtvem/runtimes.json file found")
}

// LocalVersion reads the local version for a runtime by walking up the directory tree
func LocalVersion(runtimeName string) (string, error) {
	return findLocalVersion(runtimeName)
}

// GlobalVersion reads the global version for a runtime.
// Returns ("", nil) when no version is configured (file missing or runtime not in config).
// Returns an error only for actual failures (I/O errors, corrupt JSON).
func GlobalVersion(runtimeName string) (string, error) {
	configPath := GlobalConfigPath()

	// No config file yet — valid "unconfigured" state
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", nil
	}

	version, err := readVersionFile(configPath, runtimeName)
	if err != nil && strings.Contains(err.Error(), "not found in config file") {
		// Runtime not in config — valid "unconfigured" state
		return "", nil
	}

	return version, err
}

// SetGlobalVersion sets the global version for a runtime
func SetGlobalVersion(runtimeName, version string) error {
	configPath := GlobalConfigPath()

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	// Read existing config
	config := make(RuntimesConfig)

	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err == nil {
			_ = json.Unmarshal(data, &config)
		}
	}

	// Update version for runtime
	config[runtimeName] = version

	// Write back to file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}

// SetLocalVersion sets the local version for a runtime in the current directory
func SetLocalVersion(runtimeName, version string) error {
	configDir := LocalConfigDir()
	configPath := LocalConfigPath()

	// Ensure .dtvem directory exists
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Read existing config
	config := make(RuntimesConfig)

	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err == nil {
			_ = json.Unmarshal(data, &config)
		}
	}

	// Update version for runtime
	config[runtimeName] = version

	// Write to file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0644)
}
