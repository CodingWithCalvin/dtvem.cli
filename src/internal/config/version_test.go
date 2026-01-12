package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestReadVersionFile(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		runtimeName string
		expected    string
		expectError bool
	}{
		{
			name:        "read existing runtime",
			content:     `{"python": "3.11.0", "node": "18.16.0"}`,
			runtimeName: "python",
			expected:    "3.11.0",
			expectError: false,
		},
		{
			name:        "read different runtime",
			content:     `{"python": "3.11.0", "node": "18.16.0"}`,
			runtimeName: "node",
			expected:    "18.16.0",
			expectError: false,
		},
		{
			name:        "runtime not in config",
			content:     `{"python": "3.11.0"}`,
			runtimeName: "node",
			expected:    "",
			expectError: true,
		},
		{
			name:        "invalid JSON",
			content:     `{invalid json}`,
			runtimeName: "python",
			expected:    "",
			expectError: true,
		},
		{
			name:        "empty config",
			content:     `{}`,
			runtimeName: "python",
			expected:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile := filepath.Join(t.TempDir(), "runtimes.json")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			version, err := readVersionFile(tmpFile, tt.runtimeName)

			if tt.expectError {
				if err == nil {
					t.Error("readVersionFile() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("readVersionFile() unexpected error: %v", err)
				}
				if version != tt.expected {
					t.Errorf("readVersionFile() = %q, want %q", version, tt.expected)
				}
			}
		})
	}
}

func TestReadVersionFile_FileNotFound(t *testing.T) {
	_, err := readVersionFile("/nonexistent/file.json", "python")
	if err == nil {
		t.Error("readVersionFile() with nonexistent file should return error")
	}
}

func TestReadAllRuntimes(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedLen  int
		expectedKeys []string
		expectError  bool
	}{
		{
			name:         "multiple runtimes",
			content:      `{"python": "3.11.0", "node": "18.16.0", "ruby": "3.2.0"}`,
			expectedLen:  3,
			expectedKeys: []string{"python", "node", "ruby"},
			expectError:  false,
		},
		{
			name:         "single runtime",
			content:      `{"python": "3.11.0"}`,
			expectedLen:  1,
			expectedKeys: []string{"python"},
			expectError:  false,
		},
		{
			name:         "empty config",
			content:      `{}`,
			expectedLen:  0,
			expectedKeys: []string{},
			expectError:  false,
		},
		{
			name:         "invalid JSON",
			content:      `{invalid json}`,
			expectedLen:  0,
			expectedKeys: nil,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile := filepath.Join(t.TempDir(), "runtimes.json")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}

			config, err := ReadAllRuntimes(tmpFile)

			if tt.expectError {
				if err == nil {
					t.Error("ReadAllRuntimes() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ReadAllRuntimes() unexpected error: %v", err)
				return
			}

			if len(config) != tt.expectedLen {
				t.Errorf("ReadAllRuntimes() returned %d runtimes, want %d", len(config), tt.expectedLen)
			}

			// Verify all expected keys are present
			for _, key := range tt.expectedKeys {
				if _, ok := config[key]; !ok {
					t.Errorf("ReadAllRuntimes() missing expected runtime %q", key)
				}
			}
		})
	}
}

func TestReadAllRuntimes_FileNotFound(t *testing.T) {
	_, err := ReadAllRuntimes("/nonexistent/file.json")
	if err == nil {
		t.Error("ReadAllRuntimes() with nonexistent file should return error")
	}
}

func TestReadAllRuntimes_Values(t *testing.T) {
	content := `{"python": "3.11.0", "node": "18.16.0"}`
	tmpFile := filepath.Join(t.TempDir(), "runtimes.json")
	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	config, err := ReadAllRuntimes(tmpFile)
	if err != nil {
		t.Fatalf("ReadAllRuntimes() error: %v", err)
	}

	tests := []struct {
		runtime string
		version string
	}{
		{"python", "3.11.0"},
		{"node", "18.16.0"},
	}

	for _, tt := range tests {
		t.Run(tt.runtime, func(t *testing.T) {
			version, ok := config[tt.runtime]
			if !ok {
				t.Errorf("ReadAllRuntimes() missing runtime %q", tt.runtime)
				return
			}
			if version != tt.version {
				t.Errorf("ReadAllRuntimes()[%q] = %q, want %q", tt.runtime, version, tt.version)
			}
		})
	}
}

func TestRuntimesConfig_Type(t *testing.T) {
	// Test that RuntimesConfig is a map[string]string
	var config RuntimesConfig = make(map[string]string)
	config["test"] = "1.0.0"

	if val, ok := config["test"]; !ok || val != "1.0.0" {
		t.Error("RuntimesConfig should be a map[string]string")
	}
}

// Complex tests for directory walking and version resolution

func TestFindLocalRuntimesFile_DirectoryWalking(t *testing.T) {
	// Create a temporary directory structure:
	// temp/
	//   └── project/
	//       └── subdir/
	//           └── deep/
	//               └── .dtvem/runtimes.json (this is where we'll run from)
	tmpRoot := t.TempDir()
	projectDir := filepath.Join(tmpRoot, "project")
	subDir := filepath.Join(projectDir, "subdir")
	deepDir := filepath.Join(subDir, "deep")

	if err := os.MkdirAll(deepDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create config file at project level
	configDir := filepath.Join(projectDir, ".dtvem")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create .dtvem directory: %v", err)
	}

	configPath := filepath.Join(configDir, "runtimes.json")
	configContent := `{"python": "3.11.0", "node": "18.16.0"}`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Change to deep directory
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(deepDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// FindLocalRuntimesFile should walk up and find the config
	foundPath, err := FindLocalRuntimesFile()
	if err != nil {
		t.Fatalf("FindLocalRuntimesFile() error: %v", err)
	}

	// Resolve symlinks for comparison (macOS uses /var -> /private/var symlink)
	foundPathResolved, err := filepath.EvalSymlinks(foundPath)
	if err != nil {
		t.Fatalf("Failed to resolve symlinks in found path: %v", err)
	}
	configPathResolved, err := filepath.EvalSymlinks(configPath)
	if err != nil {
		t.Fatalf("Failed to resolve symlinks in config path: %v", err)
	}

	if foundPathResolved != configPathResolved {
		t.Errorf("FindLocalRuntimesFile() = %q, want %q", foundPathResolved, configPathResolved)
	}
}

func TestFindLocalRuntimesFile_TraversesThroughGitRoot(t *testing.T) {
	// Create structure:
	// temp/
	//   └── outer/
	//       └── .dtvem/runtimes.json
	//       └── repo/
	//           └── .git/
	//           └── subdir/
	//               (run from here - SHOULD find outer config)
	tmpRoot := t.TempDir()
	outerDir := filepath.Join(tmpRoot, "outer")
	repoDir := filepath.Join(outerDir, "repo")
	subDir := filepath.Join(repoDir, "subdir")

	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create directory structure: %v", err)
	}

	// Create outer config (SHOULD be found)
	outerConfigDir := filepath.Join(outerDir, ".dtvem")
	if err := os.MkdirAll(outerConfigDir, 0755); err != nil {
		t.Fatalf("Failed to create outer .dtvem: %v", err)
	}
	outerConfigPath := filepath.Join(outerConfigDir, "runtimes.json")
	if err := os.WriteFile(outerConfigPath, []byte(`{"python": "3.11.0"}`), 0644); err != nil {
		t.Fatalf("Failed to write outer config: %v", err)
	}

	// Create .git directory at repo level (should NOT stop traversal)
	gitDir := filepath.Join(repoDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git directory: %v", err)
	}

	// Change to subdir and try to find config
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(subDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// SHOULD find the outer config (traverses through git root)
	foundPath, err := FindLocalRuntimesFile()
	if err != nil {
		t.Fatalf("FindLocalRuntimesFile() error: %v", err)
	}

	// Resolve symlinks for comparison (macOS uses /var -> /private/var symlink)
	foundPathResolved, err := filepath.EvalSymlinks(foundPath)
	if err != nil {
		t.Fatalf("Failed to resolve symlinks in found path: %v", err)
	}
	outerConfigPathResolved, err := filepath.EvalSymlinks(outerConfigPath)
	if err != nil {
		t.Fatalf("Failed to resolve symlinks in outer config path: %v", err)
	}

	if foundPathResolved != outerConfigPathResolved {
		t.Errorf("FindLocalRuntimesFile() = %q, want %q", foundPathResolved, outerConfigPathResolved)
	}
}

func TestFindLocalRuntimesFile_NoConfigFound(t *testing.T) {
	// Create empty directory structure with no config
	tmpRoot := t.TempDir()
	testDir := filepath.Join(tmpRoot, "test")

	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(testDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	_, err := FindLocalRuntimesFile()
	if err == nil {
		t.Error("FindLocalRuntimesFile() should return error when no config found")
	}
}

func TestSetGlobalVersion_CreatesFile(t *testing.T) {
	// Use temp directory for global config
	tmpRoot := t.TempDir()

	// Set HOME to temp for this test
	t.Setenv("HOME", tmpRoot)
	t.Setenv("USERPROFILE", tmpRoot)

	// Set a global version
	err := SetGlobalVersion("python", "3.11.0")
	if err != nil {
		t.Fatalf("SetGlobalVersion() error: %v", err)
	}

	// Verify file was created and contains correct content
	actualConfigPath := GlobalConfigPath()
	data, err := os.ReadFile(actualConfigPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config RuntimesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if config["python"] != "3.11.0" {
		t.Errorf("Config python version = %q, want %q", config["python"], "3.11.0")
	}
}

func TestSetGlobalVersion_UpdatesExisting(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("HOME", tmpRoot)
	t.Setenv("USERPROFILE", tmpRoot)

	// Set initial version
	if err := SetGlobalVersion("python", "3.11.0"); err != nil {
		t.Fatalf("SetGlobalVersion() initial error: %v", err)
	}

	// Update to new version
	if err := SetGlobalVersion("python", "3.12.0"); err != nil {
		t.Fatalf("SetGlobalVersion() update error: %v", err)
	}

	// Verify it was updated
	version, err := GlobalVersion("python")
	if err != nil {
		t.Fatalf("GlobalVersion() error: %v", err)
	}

	if version != "3.12.0" {
		t.Errorf("GlobalVersion() = %q, want %q", version, "3.12.0")
	}
}

func TestSetGlobalVersion_MultipleRuntimes(t *testing.T) {
	tmpRoot := t.TempDir()
	t.Setenv("HOME", tmpRoot)
	t.Setenv("USERPROFILE", tmpRoot)

	// Set versions for multiple runtimes
	runtimes := map[string]string{
		"python": "3.11.0",
		"node":   "18.16.0",
		"ruby":   "3.2.0",
	}

	for runtime, version := range runtimes {
		if err := SetGlobalVersion(runtime, version); err != nil {
			t.Fatalf("SetGlobalVersion(%q, %q) error: %v", runtime, version, err)
		}
	}

	// Verify all were saved
	for runtime, expectedVersion := range runtimes {
		version, err := GlobalVersion(runtime)
		if err != nil {
			t.Errorf("GlobalVersion(%q) error: %v", runtime, err)
			continue
		}
		if version != expectedVersion {
			t.Errorf("GlobalVersion(%q) = %q, want %q", runtime, version, expectedVersion)
		}
	}
}

func TestSetLocalVersion_CreatesDirectoryAndFile(t *testing.T) {
	// Create temp directory and change to it
	tmpRoot := t.TempDir()
	originalDir, _ := os.Getwd()
	defer func() { _ = os.Chdir(originalDir) }()

	if err := os.Chdir(tmpRoot); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Set local version (should create .dtvem/runtimes.json)
	err := SetLocalVersion("python", "3.11.0")
	if err != nil {
		t.Fatalf("SetLocalVersion() error: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpRoot, ".dtvem", "runtimes.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("SetLocalVersion() did not create config file")
	}

	// Verify content
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	var config RuntimesConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	if config["python"] != "3.11.0" {
		t.Errorf("Config python version = %q, want %q", config["python"], "3.11.0")
	}
}
