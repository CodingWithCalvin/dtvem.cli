package migration

import (
	"testing"
)

// mockProvider is a minimal test implementation of the Provider interface.
type mockProvider struct {
	name             string
	displayName      string
	runtime          string
	present          bool
	canAutoUninstall bool
}

func (m *mockProvider) Name() string                               { return m.name }
func (m *mockProvider) DisplayName() string                        { return m.displayName }
func (m *mockProvider) Runtime() string                            { return m.runtime }
func (m *mockProvider) IsPresent() bool                            { return m.present }
func (m *mockProvider) DetectVersions() ([]DetectedVersion, error) { return nil, nil }
func (m *mockProvider) CanAutoUninstall() bool                     { return m.canAutoUninstall }
func (m *mockProvider) UninstallCommand(version string) string     { return "" }
func (m *mockProvider) ManualInstructions() string                 { return "" }

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
				{name: "test", displayName: "Test", runtime: "node"},
			},
			expectError: false,
		},
		{
			name: "register multiple providers",
			providers: []*mockProvider{
				{name: "test1", displayName: "Test 1", runtime: "node"},
				{name: "test2", displayName: "Test 2", runtime: "python"},
			},
			expectError: false,
		},
		{
			name: "register duplicate provider",
			providers: []*mockProvider{
				{name: "test", displayName: "Test 1", runtime: "node"},
				{name: "test", displayName: "Test 2", runtime: "node"},
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
	provider := &mockProvider{name: "test", displayName: "Test", runtime: "node"}
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

func TestRegistry_GetByRuntime(t *testing.T) {
	r := NewRegistry()

	providers := []*mockProvider{
		{name: "nvm", displayName: "nvm", runtime: "node"},
		{name: "fnm", displayName: "fnm", runtime: "node"},
		{name: "pyenv", displayName: "pyenv", runtime: "python"},
		{name: "rbenv", displayName: "rbenv", runtime: "ruby"},
	}

	for _, p := range providers {
		if err := r.Register(p); err != nil {
			t.Fatalf("Failed to register provider: %v", err)
		}
	}

	tests := []struct {
		name     string
		runtime  string
		expected int
	}{
		{
			name:     "get node providers",
			runtime:  "node",
			expected: 2,
		},
		{
			name:     "get python providers",
			runtime:  "python",
			expected: 1,
		},
		{
			name:     "get ruby providers",
			runtime:  "ruby",
			expected: 1,
		},
		{
			name:     "get non-existent runtime",
			runtime:  "golang",
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.GetByRuntime(tt.runtime)
			if len(result) != tt.expected {
				t.Errorf("GetByRuntime(%q) returned %d providers, want %d", tt.runtime, len(result), tt.expected)
			}

			// Verify all returned providers are for the correct runtime
			for _, p := range result {
				if p.Runtime() != tt.runtime {
					t.Errorf("GetByRuntime(%q) returned provider with runtime %q", tt.runtime, p.Runtime())
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
				{name: "test", displayName: "Test", runtime: "node"},
			},
			expected: 1,
		},
		{
			name: "multiple providers",
			providers: []*mockProvider{
				{name: "test1", displayName: "Test 1", runtime: "node"},
				{name: "test2", displayName: "Test 2", runtime: "python"},
				{name: "test3", displayName: "Test 3", runtime: "ruby"},
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
				{name: "test1", displayName: "Test 1", runtime: "node"},
				{name: "test2", displayName: "Test 2", runtime: "python"},
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
	provider := &mockProvider{name: "test", displayName: "Test", runtime: "node"}
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
		setup       func(*Registry)
		unregister  string
		expectError bool
	}{
		{
			name: "unregister existing provider",
			setup: func(r *Registry) {
				_ = r.Register(&mockProvider{name: "test", displayName: "Test", runtime: "node"})
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
	provider := &mockProvider{name: "test", displayName: "Test", runtime: "node"}

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
		r.GetByRuntime("node")
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

func TestDetectedVersion_String(t *testing.T) {
	dv := DetectedVersion{
		Version:   "22.0.0",
		Path:      "/path/to/node",
		Source:    "nvm",
		Validated: true,
	}

	result := dv.String()
	expected := "v22.0.0 (nvm) /path/to/node"

	if result != expected {
		t.Errorf("DetectedVersion.String() = %q, want %q", result, expected)
	}
}
