package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSettingsPath(t *testing.T) {
	result := SettingsPath()

	// Should not be empty
	if result == "" {
		t.Error("SettingsPath() returned empty string")
	}

	// Should end with settings.json
	if !hasSettingsSuffix(result) {
		t.Errorf("SettingsPath() = %q, should end with %q", result, SettingsFileName)
	}

	// Should be absolute path
	if !filepath.IsAbs(result) {
		t.Errorf("SettingsPath() = %q, should be absolute", result)
	}
}

func hasSettingsSuffix(path string) bool {
	return filepath.Base(path) == SettingsFileName
}

func TestLoadSettings_FileNotExists(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		resetPathsForTesting()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	resetPathsForTesting()

	// LoadSettings should return default settings when file doesn't exist
	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}

	if settings == nil {
		t.Fatal("LoadSettings() returned nil settings")
	}

	// Default should be system install type
	if settings.InstallType != InstallTypeSystem {
		t.Errorf("LoadSettings() default InstallType = %q, want %q",
			settings.InstallType, InstallTypeSystem)
	}
}

func TestSaveAndLoadSettings(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		resetPathsForTesting()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	resetPathsForTesting()

	// Test saving and loading system install type
	t.Run("system install type", func(t *testing.T) {
		settings := &Settings{InstallType: InstallTypeSystem}
		if err := SaveSettings(settings); err != nil {
			t.Fatalf("SaveSettings() error = %v", err)
		}

		loaded, err := LoadSettings()
		if err != nil {
			t.Fatalf("LoadSettings() error = %v", err)
		}

		if loaded.InstallType != InstallTypeSystem {
			t.Errorf("LoadSettings() InstallType = %q, want %q",
				loaded.InstallType, InstallTypeSystem)
		}
	})

	// Test saving and loading user install type
	t.Run("user install type", func(t *testing.T) {
		settings := &Settings{InstallType: InstallTypeUser}
		if err := SaveSettings(settings); err != nil {
			t.Fatalf("SaveSettings() error = %v", err)
		}

		loaded, err := LoadSettings()
		if err != nil {
			t.Fatalf("LoadSettings() error = %v", err)
		}

		if loaded.InstallType != InstallTypeUser {
			t.Errorf("LoadSettings() InstallType = %q, want %q",
				loaded.InstallType, InstallTypeUser)
		}
	})
}

func TestLoadSettings_InvalidInstallType(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		resetPathsForTesting()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	resetPathsForTesting()

	// Create config directory and settings file with invalid install type
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	settingsPath := filepath.Join(configDir, SettingsFileName)
	invalidJSON := `{"installType": "invalid"}`
	if err := os.WriteFile(settingsPath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to write test settings file: %v", err)
	}

	// LoadSettings should return default install type for invalid values
	settings, err := LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings() unexpected error: %v", err)
	}

	// Should default to system for invalid values
	if settings.InstallType != InstallTypeSystem {
		t.Errorf("LoadSettings() with invalid value should default to %q, got %q",
			InstallTypeSystem, settings.InstallType)
	}
}

func TestLoadSettings_MalformedJSON(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		resetPathsForTesting()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	resetPathsForTesting()

	// Create config directory and settings file with malformed JSON
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config directory: %v", err)
	}

	settingsPath := filepath.Join(configDir, SettingsFileName)
	malformedJSON := `{not valid json`
	if err := os.WriteFile(settingsPath, []byte(malformedJSON), 0644); err != nil {
		t.Fatalf("Failed to write test settings file: %v", err)
	}

	// LoadSettings should return an error for malformed JSON
	_, err := LoadSettings()
	if err == nil {
		t.Error("LoadSettings() with malformed JSON should return an error")
	}
}

func TestIsUserInstall(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		resetPathsForTesting()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	resetPathsForTesting()

	// Test with no settings file (should return false)
	t.Run("no settings file", func(t *testing.T) {
		if IsUserInstall() {
			t.Error("IsUserInstall() with no settings file should return false")
		}
	})

	// Test with system install type
	t.Run("system install type", func(t *testing.T) {
		settings := &Settings{InstallType: InstallTypeSystem}
		if err := SaveSettings(settings); err != nil {
			t.Fatalf("SaveSettings() error = %v", err)
		}

		if IsUserInstall() {
			t.Error("IsUserInstall() with system install type should return false")
		}
	})

	// Test with user install type
	t.Run("user install type", func(t *testing.T) {
		settings := &Settings{InstallType: InstallTypeUser}
		if err := SaveSettings(settings); err != nil {
			t.Fatalf("SaveSettings() error = %v", err)
		}

		if !IsUserInstall() {
			t.Error("IsUserInstall() with user install type should return true")
		}
	})
}

func TestSaveSettings_CreatesConfigDirectory(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		resetPathsForTesting()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	resetPathsForTesting()

	// Config directory should not exist initially
	configDir := filepath.Join(tmpDir, "config")
	if _, err := os.Stat(configDir); err == nil {
		t.Fatal("Config directory should not exist initially")
	}

	// SaveSettings should create the config directory
	settings := &Settings{InstallType: InstallTypeUser}
	if err := SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings() error = %v", err)
	}

	// Config directory should now exist
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		t.Error("SaveSettings() should create config directory")
	}

	// Settings file should exist
	settingsPath := filepath.Join(configDir, SettingsFileName)
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("SaveSettings() should create settings file")
	}
}

func TestInstallTypeConstants(t *testing.T) {
	// Verify constant values
	if InstallTypeSystem != "system" {
		t.Errorf("InstallTypeSystem = %q, want %q", InstallTypeSystem, "system")
	}
	if InstallTypeUser != "user" {
		t.Errorf("InstallTypeUser = %q, want %q", InstallTypeUser, "user")
	}
}

func TestSettingsFileName(t *testing.T) {
	expected := "settings.json"
	if SettingsFileName != expected {
		t.Errorf("SettingsFileName = %q, want %q", SettingsFileName, expected)
	}
}
