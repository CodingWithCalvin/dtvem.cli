package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RuntimesConfig represents the flat structure of runtimes.json
// Format: {"python": "3.11.0", "node": "18.16.0"}
type RuntimesConfig map[string]string

// SchemaURL is the URL to the runtimes.json schema
const SchemaURL = "https://raw.githubusercontent.com/CodingWithCalvin/dtvem.cli/main/schemas/runtimes.schema.json"

// ErrNoVersion reports that no version is configured for a runtime, either
// for the working directory or globally.
var ErrNoVersion = errors.New("no version configured")

// Resolution is the version a runtime resolves to and where it came from.
type Resolution struct {
	// Version is the version to use. When a version file asks for a partial
	// version ("22"), it is the highest installed match, or the partial
	// version itself if nothing installed matches.
	Version string
	// Source is the path of the file that set the version.
	Source string
}

// Resolve finds the version to use for a runtime and the file that sets it.
// A version set for the working directory or one of its parents (see
// findLocal) wins over the global one. The error wraps ErrNoVersion when
// neither sets a version, and is a *VersionFileError when the nearest
// version file asks for something dtvem cannot use. Any other error means
// the working directory or a version file could not be read.
func Resolve(runtimeName string) (Resolution, error) {
	if res, found, err := findLocal(runtimeName); err != nil || found {
		return res, err
	}

	if globalVersion, err := GlobalVersion(runtimeName); err == nil && globalVersion != "" {
		return Resolution{Version: globalVersion, Source: GlobalConfigPath()}, nil
	}

	return Resolution{}, fmt.Errorf("%w for %s", ErrNoVersion, runtimeName)
}

// ResolveVersion finds the version to use for a runtime. See Resolve.
func ResolveVersion(runtimeName string) (string, error) {
	res, err := Resolve(runtimeName)
	return res.Version, err
}

// findLocal walks up from the working directory to the filesystem root and
// returns the version set by the nearest directory that sets one for
// runtimeName. The bool is false when no directory does.
func findLocal(runtimeName string) (Resolution, bool, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return Resolution{}, false, fmt.Errorf("could not determine the working directory: %w", err)
	}

	for {
		if res, found, err := resolveInDir(currentDir, runtimeName); err != nil || found {
			return res, found, err
		}

		parent := filepath.Dir(currentDir)
		if parent == currentDir {
			return Resolution{}, false, nil
		}
		currentDir = parent
	}
}

// resolveInDir returns the version dir sets for runtimeName, if any. It
// checks .dtvem/runtimes.json first, then the runtime's version files (see
// versionFiles). A runtimes.json that is unreadable or lacks the runtime is
// passed over; a version file that exists but cannot be used is an error,
// so a project's pin is never silently replaced by another version.
func resolveInDir(dir, runtimeName string) (Resolution, bool, error) {
	runtimesFile := filepath.Join(dir, LocalConfigDirName, RuntimesFileName)
	if version, err := readVersionFile(runtimesFile, runtimeName); err == nil && version != "" {
		return Resolution{Version: version, Source: runtimesFile}, true, nil
	}

	for _, name := range versionFiles[runtimeName] {
		path := filepath.Join(dir, name)
		spec, err := readVersionSpec(path)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return Resolution{}, false, err
		}
		return Resolution{Version: resolveSpec(runtimeName, spec), Source: path}, true, nil
	}

	return Resolution{}, false, nil
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

// LocalVersion reads the local version for a runtime by walking up the
// directory tree. See findLocal.
func LocalVersion(runtimeName string) (string, error) {
	res, found, err := findLocal(runtimeName)
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("no local version file found")
	}
	return res.Version, nil
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
