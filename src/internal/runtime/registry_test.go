package runtime

import (
	"testing"
)

// mockProvider is a minimal test implementation of the Provider interface
type mockProvider struct {
	name        string
	displayName string
}

func (m *mockProvider) Name() string        { return m.name }
func (m *mockProvider) DisplayName() string { return m.displayName }
func (m *mockProvider) Shims() []string     { return []string{m.name} }
func (m *mockProvider) Install(version string) error                                  { return nil }
func (m *mockProvider) Uninstall(version string) error                                { return nil }
func (m *mockProvider) ListInstalled() ([]InstalledVersion, error)                    { return nil, nil }
func (m *mockProvider) ListAvailable() ([]AvailableVersion, error)                    { return nil, nil }
func (m *mockProvider) ExecutablePath(version string) (string, error)                 { return "", nil }
func (m *mockProvider) IsInstalled(version string) (bool, error)                      { return false, nil }
func (m *mockProvider) InstallPath(version string) (string, error)                    { return "", nil }
func (m *mockProvider) GlobalVersion() (string, error)                                { return "", nil }
func (m *mockProvider) SetGlobalVersion(version string) error                         { return nil }
func (m *mockProvider) LocalVersion() (string, error)                                 { return "", nil }
func (m *mockProvider) SetLocalVersion(version string) error                          { return nil }
func (m *mockProvider) CurrentVersion() (string, error)                               { return "", nil }
func (m *mockProvider) DetectInstalled() ([]DetectedVersion, error)                   { return nil, nil }
func (m *mockProvider) GlobalPackages(installPath string) ([]string, error)           { return nil, nil }
func (m *mockProvider) InstallGlobalPackages(version string, packages []string) error { return nil }
func (m *mockProvider) ManualPackageInstallCommand(packages []string) string          { return "" }
func (m *mockProvider) ShouldReshimAfter(shimName string, args []string) bool         { return false }

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry() returned nil")
	}
	if r.providers == nil {
		t.Error("NewRegistry() did not initialize providers map")
	}
	if len(r.providers) != 0 {
		t.Errorf("NewRegistry() providers map has %d entries, want 0", len(r.providers))
	}
}

func TestRegistry_Register(t *testing.T) {
	tests := []struct {
		name        string
		providers   []*mockProvider
		expectError bool
	}{
		{
			name: "register single provider",
			providers: []*mockProvider{
				{name: "test", displayName: "Test"},
			},
			expectError: false,
		},
		{
			name: "register multiple providers",
			providers: []*mockProvider{
				{name: "test1", displayName: "Test 1"},
				{name: "test2", displayName: "Test 2"},
			},
			expectError: false,
		},
		{
			name: "register duplicate provider",
			providers: []*mockProvider{
				{name: "test", displayName: "Test 1"},
				{name: "test", displayName: "Test 2"},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			var err error
			for _, p := range tt.providers {
				err = r.Register(p)
			}

			if tt.expectError && err == nil {
				t.Error("Register() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Register() unexpected error: %v", err)
			}
		})
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	provider := &mockProvider{name: "test", displayName: "Test"}
	if err := r.Register(provider); err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	tests := []struct {
		name        string
		searchName  string
		expectError bool
	}{
		{
			name:        "get existing provider",
			searchName:  "test",
			expectError: false,
		},
		{
			name:        "get non-existent provider",
			searchName:  "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := r.Get(tt.searchName)

			if tt.expectError {
				if err == nil {
					t.Error("Get() expected error, got nil")
				}
				if p != nil {
					t.Error("Get() expected nil provider on error")
				}
			} else {
				if err != nil {
					t.Errorf("Get() unexpected error: %v", err)
				}
				if p == nil {
					t.Error("Get() returned nil provider")
				}
				if p.Name() != tt.searchName {
					t.Errorf("Get() returned provider with name %q, want %q", p.Name(), tt.searchName)
				}
			}
		})
	}
}

func TestRegistry_List(t *testing.T) {
	tests := []struct {
		name      string
		providers []*mockProvider
		expected  int
	}{
		{
			name:      "empty registry",
			providers: []*mockProvider{},
			expected:  0,
		},
		{
			name: "single provider",
			providers: []*mockProvider{
				{name: "test", displayName: "Test"},
			},
			expected: 1,
		},
		{
			name: "multiple providers",
			providers: []*mockProvider{
				{name: "test1", displayName: "Test 1"},
				{name: "test2", displayName: "Test 2"},
				{name: "test3", displayName: "Test 3"},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			for _, p := range tt.providers {
				if err := r.Register(p); err != nil {
					t.Fatalf("Failed to register provider: %v", err)
				}
			}

			list := r.List()
			if len(list) != tt.expected {
				t.Errorf("List() returned %d names, want %d", len(list), tt.expected)
			}

			// Verify all registered providers are in the list
			for _, p := range tt.providers {
				found := false
				for _, name := range list {
					if name == p.name {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("List() missing provider %q", p.name)
				}
			}
		})
	}
}

func TestRegistry_GetAll(t *testing.T) {
	tests := []struct {
		name      string
		providers []*mockProvider
		expected  int
	}{
		{
			name:      "empty registry",
			providers: []*mockProvider{},
			expected:  0,
		},
		{
			name: "multiple providers",
			providers: []*mockProvider{
				{name: "test1", displayName: "Test 1"},
				{name: "test2", displayName: "Test 2"},
			},
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			for _, p := range tt.providers {
				if err := r.Register(p); err != nil {
					t.Fatalf("Failed to register provider: %v", err)
				}
			}

			all := r.GetAll()
			if len(all) != tt.expected {
				t.Errorf("GetAll() returned %d providers, want %d", len(all), tt.expected)
			}
		})
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()
	provider := &mockProvider{name: "test", displayName: "Test"}
	if err := r.Register(provider); err != nil {
		t.Fatalf("Failed to register provider: %v", err)
	}

	tests := []struct {
		name       string
		searchName string
		expected   bool
	}{
		{
			name:       "existing provider",
			searchName: "test",
			expected:   true,
		},
		{
			name:       "non-existent provider",
			searchName: "nonexistent",
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.Has(tt.searchName)
			if result != tt.expected {
				t.Errorf("Has(%q) = %v, want %v", tt.searchName, result, tt.expected)
			}
		})
	}
}

func TestRegistry_Unregister(t *testing.T) {
	tests := []struct {
		name        string
		setup       func(*Registry) // Setup function to prepare registry
		unregister  string          // Provider name to unregister
		expectError bool
	}{
		{
			name: "unregister existing provider",
			setup: func(r *Registry) {
				_ = r.Register(&mockProvider{name: "test", displayName: "Test"})
			},
			unregister:  "test",
			expectError: false,
		},
		{
			name:        "unregister non-existent provider",
			setup:       func(r *Registry) {},
			unregister:  "nonexistent",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRegistry()
			tt.setup(r)

			err := r.Unregister(tt.unregister)

			if tt.expectError && err == nil {
				t.Error("Unregister() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unregister() unexpected error: %v", err)
			}

			// Verify provider is actually removed
			if !tt.expectError && r.Has(tt.unregister) {
				t.Errorf("Unregister() did not remove provider %q", tt.unregister)
			}
		})
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	// This test verifies that the registry is thread-safe
	r := NewRegistry()
	provider := &mockProvider{name: "test", displayName: "Test"}

	// Register in one goroutine
	done := make(chan bool)
	go func() {
		_ = r.Register(provider)
		done <- true
	}()

	// Read in another goroutine
	go func() {
		r.Has("test")
		r.List()
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify the provider was registered
	if !r.Has("test") {
		t.Error("Concurrent Register() did not work correctly")
	}
}
