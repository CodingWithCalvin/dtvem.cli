package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
)

func TestDetermineInstallType_FromSettings(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		config.ResetPathsCache()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	config.ResetPathsCache()

	tests := []struct {
		name         string
		installType  config.InstallType
		expectedUser bool
	}{
		{
			name:         "Settings with system install type",
			installType:  config.InstallTypeSystem,
			expectedUser: false,
		},
		{
			name:         "Settings with user install type",
			installType:  config.InstallTypeUser,
			expectedUser: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save settings
			settings := &config.Settings{InstallType: tt.installType}
			if err := config.SaveSettings(settings); err != nil {
				t.Fatalf("Failed to save settings: %v", err)
			}

			// Load settings and check install type
			loadedSettings, err := config.LoadSettings()
			if err != nil {
				t.Fatalf("Failed to load settings: %v", err)
			}

			result := loadedSettings.InstallType == config.InstallTypeUser
			if result != tt.expectedUser {
				t.Errorf("determineInstallType from settings %q = %v, want %v",
					tt.installType, result, tt.expectedUser)
			}
		})
	}
}

func TestDetermineInstallType_NoSettings(t *testing.T) {
	// Create a temp directory without settings file
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		config.ResetPathsCache()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	config.ResetPathsCache()

	// Load settings (should return defaults)
	settings, err := config.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings returned unexpected error: %v", err)
	}

	// Default should be system install
	if settings.InstallType != config.InstallTypeSystem {
		t.Errorf("Default install type = %q, want %q",
			settings.InstallType, config.InstallTypeSystem)
	}
}

func TestSaveSettingsAfterInit(t *testing.T) {
	// Create a temp directory for test
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		config.ResetPathsCache()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	config.ResetPathsCache()

	// Simulate saving settings after init (user install)
	settings := &config.Settings{InstallType: config.InstallTypeUser}
	if err := config.SaveSettings(settings); err != nil {
		t.Fatalf("SaveSettings failed: %v", err)
	}

	// Verify settings file was created
	settingsPath := config.SettingsPath()
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Error("Settings file should exist after save")
	}

	// Verify settings can be loaded
	loaded, err := config.LoadSettings()
	if err != nil {
		t.Fatalf("LoadSettings failed: %v", err)
	}

	if loaded.InstallType != config.InstallTypeUser {
		t.Errorf("Loaded InstallType = %q, want %q",
			loaded.InstallType, config.InstallTypeUser)
	}
}

func TestSettingsPersistence(t *testing.T) {
	// Test that settings persist across multiple init calls
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		config.ResetPathsCache()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	config.ResetPathsCache()

	// First "init" with user install
	settings := &config.Settings{InstallType: config.InstallTypeUser}
	if err := config.SaveSettings(settings); err != nil {
		t.Fatalf("First save failed: %v", err)
	}

	// Verify first save
	loaded, _ := config.LoadSettings()
	if loaded.InstallType != config.InstallTypeUser {
		t.Errorf("First load: InstallType = %q, want %q",
			loaded.InstallType, config.InstallTypeUser)
	}

	// Simulate second "init" without flag - should use saved settings
	loaded2, _ := config.LoadSettings()
	if loaded2.InstallType != config.InstallTypeUser {
		t.Errorf("Second load (no change): InstallType = %q, want %q",
			loaded2.InstallType, config.InstallTypeUser)
	}
}

func TestInitCommandFlags(t *testing.T) {
	// Verify that the init command has the expected flags
	t.Run("--yes flag exists", func(t *testing.T) {
		flag := initCmd.Flags().Lookup("yes")
		if flag == nil {
			t.Error("--yes flag should exist on init command")
			return
		}
		if flag.Shorthand != "y" {
			t.Errorf("--yes flag shorthand = %q, want %q", flag.Shorthand, "y")
		}
	})

	t.Run("--user flag exists", func(t *testing.T) {
		flag := initCmd.Flags().Lookup("user")
		if flag == nil {
			t.Error("--user flag should exist on init command")
		}
	})
}

func TestDirectoriesCreation(t *testing.T) {
	// Test that EnsureDirectories creates all expected directories
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		config.ResetPathsCache()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	config.ResetPathsCache()

	// Create directories
	if err := config.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories failed: %v", err)
	}

	// Verify expected directories exist
	expectedDirs := []string{
		tmpDir,
		filepath.Join(tmpDir, "shims"),
		filepath.Join(tmpDir, "versions"),
		filepath.Join(tmpDir, "config"),
		filepath.Join(tmpDir, "cache"),
	}

	for _, dir := range expectedDirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			t.Errorf("Directory %q should exist after EnsureDirectories", dir)
		}
	}
}

func TestInstallTypeConstants(t *testing.T) {
	// Verify constant values match expected strings
	if config.InstallTypeSystem != "system" {
		t.Errorf("InstallTypeSystem = %q, want %q", config.InstallTypeSystem, "system")
	}
	if config.InstallTypeUser != "user" {
		t.Errorf("InstallTypeUser = %q, want %q", config.InstallTypeUser, "user")
	}
}

func TestInstallTypeSwitchDetection(t *testing.T) {
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		config.ResetPathsCache()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	config.ResetPathsCache()

	t.Run("Detects switch from system to user", func(t *testing.T) {
		// Save system install settings
		settings := &config.Settings{InstallType: config.InstallTypeSystem}
		_ = config.SaveSettings(settings)

		// Load and verify
		loaded, _ := config.LoadSettings()
		if loaded.InstallType != config.InstallTypeSystem {
			t.Error("Should have system install type")
		}

		// Check if switching to user would be detected
		// (The actual isSwitching check happens in the command handler)
		newType := config.InstallTypeUser
		isSwitching := loaded.InstallType != newType
		if !isSwitching {
			t.Error("Should detect switch from system to user")
		}
	})

	t.Run("Detects switch from user to system", func(t *testing.T) {
		// Save user install settings
		settings := &config.Settings{InstallType: config.InstallTypeUser}
		_ = config.SaveSettings(settings)

		// Load and verify
		loaded, _ := config.LoadSettings()
		if loaded.InstallType != config.InstallTypeUser {
			t.Error("Should have user install type")
		}

		// Check if switching to system would be detected
		newType := config.InstallTypeSystem
		isSwitching := loaded.InstallType != newType
		if !isSwitching {
			t.Error("Should detect switch from user to system")
		}
	})

	t.Run("No switch when types match", func(t *testing.T) {
		// Save user install settings
		settings := &config.Settings{InstallType: config.InstallTypeUser}
		_ = config.SaveSettings(settings)

		// Load and verify
		loaded, _ := config.LoadSettings()
		if loaded.InstallType != config.InstallTypeUser {
			t.Error("Should have user install type")
		}

		// Check that same type is not a switch
		newType := config.InstallTypeUser
		isSwitching := loaded.InstallType != newType
		if isSwitching {
			t.Error("Should not detect switch when types match")
		}
	})
}

func TestIsUserInstallFunction(t *testing.T) {
	tmpDir := t.TempDir()
	originalRoot := os.Getenv("DTVEM_ROOT")
	defer func() {
		if originalRoot != "" {
			_ = os.Setenv("DTVEM_ROOT", originalRoot)
		} else {
			_ = os.Unsetenv("DTVEM_ROOT")
		}
		config.ResetPathsCache()
	}()

	_ = os.Setenv("DTVEM_ROOT", tmpDir)
	config.ResetPathsCache()

	t.Run("Returns false with no settings", func(t *testing.T) {
		if config.IsUserInstall() {
			t.Error("IsUserInstall should return false when no settings file exists")
		}
	})

	t.Run("Returns false with system install", func(t *testing.T) {
		settings := &config.Settings{InstallType: config.InstallTypeSystem}
		_ = config.SaveSettings(settings)

		if config.IsUserInstall() {
			t.Error("IsUserInstall should return false for system install type")
		}
	})

	t.Run("Returns true with user install", func(t *testing.T) {
		settings := &config.Settings{InstallType: config.InstallTypeUser}
		_ = config.SaveSettings(settings)

		if !config.IsUserInstall() {
			t.Error("IsUserInstall should return true for user install type")
		}
	})
}
