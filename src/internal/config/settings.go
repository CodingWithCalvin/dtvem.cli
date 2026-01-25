package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// InstallType represents the type of dtvem installation
type InstallType string

const (
	// InstallTypeSystem uses System PATH (requires admin on Windows)
	InstallTypeSystem InstallType = "system"
	// InstallTypeUser uses User PATH (no admin required)
	InstallTypeUser InstallType = "user"
)

// SettingsFileName is the name of the settings configuration file
const SettingsFileName = "settings.json"

// Settings holds dtvem installation settings
type Settings struct {
	InstallType InstallType `json:"installType"`
}

// SettingsPath returns the path to the settings file
func SettingsPath() string {
	paths := DefaultPaths()
	return filepath.Join(paths.Config, SettingsFileName)
}

// LoadSettings loads settings from the settings file.
// Returns default settings (system install type) if the file doesn't exist.
func LoadSettings() (*Settings, error) {
	settingsPath := SettingsPath()

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Return default settings if file doesn't exist
			return &Settings{InstallType: InstallTypeSystem}, nil
		}
		return nil, err
	}

	var settings Settings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, err
	}

	// Validate install type
	if settings.InstallType != InstallTypeSystem && settings.InstallType != InstallTypeUser {
		// Default to system if invalid value
		settings.InstallType = InstallTypeSystem
	}

	return &settings, nil
}

// SaveSettings saves settings to the settings file
func SaveSettings(settings *Settings) error {
	settingsPath := SettingsPath()

	// Ensure the config directory exists
	configDir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsPath, data, 0644)
}

// IsUserInstall checks if the current installation is a user-level install
func IsUserInstall() bool {
	settings, err := LoadSettings()
	if err != nil {
		return false
	}
	return settings.InstallType == InstallTypeUser
}
