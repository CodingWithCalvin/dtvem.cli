package migration

import (
	"fmt"
	"sync"
)

// Registry manages all registered migration providers.
type Registry struct {
	providers map[string]Provider
	mu        sync.RWMutex
}

// Global registry instance
var globalRegistry = &Registry{
	providers: make(map[string]Provider),
}

// NewRegistry creates a new migration registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]Provider),
	}
}

// Register adds a migration provider to the registry.
func (r *Registry) Register(provider Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("migration provider '%s' is already registered", name)
	}

	r.providers[name] = provider
	return nil
}

// Get retrieves a migration provider by name.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[name]
	if !exists {
		return nil, fmt.Errorf("migration provider '%s' not found", name)
	}

	return provider, nil
}

// GetByRuntime returns all migration providers for a given runtime.
func (r *Registry) GetByRuntime(runtimeName string) []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0)
	for _, provider := range r.providers {
		if provider.Runtime() == runtimeName {
			providers = append(providers, provider)
		}
	}
	return providers
}

// List returns all registered migration provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	return names
}

// GetAll returns all registered providers.
func (r *Registry) GetAll() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]Provider, 0, len(r.providers))
	for _, provider := range r.providers {
		providers = append(providers, provider)
	}
	return providers
}

// Has checks if a migration provider is registered.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.providers[name]
	return exists
}

// Unregister removes a migration provider from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; !exists {
		return fmt.Errorf("migration provider '%s' not found", name)
	}

	delete(r.providers, name)
	return nil
}

// Global registry access functions

// Register adds a provider to the global registry.
func Register(provider Provider) error {
	return globalRegistry.Register(provider)
}

// Get retrieves a provider from the global registry.
func Get(name string) (Provider, error) {
	return globalRegistry.Get(name)
}

// GetByRuntime returns all providers for a runtime from the global registry.
func GetByRuntime(runtimeName string) []Provider {
	return globalRegistry.GetByRuntime(runtimeName)
}

// List returns all registered provider names from the global registry.
func List() []string {
	return globalRegistry.List()
}

// GetAll returns all providers from the global registry.
func GetAll() []Provider {
	return globalRegistry.GetAll()
}

// Has checks if a provider exists in the global registry.
func Has(name string) bool {
	return globalRegistry.Has(name)
}

// Unregister removes a provider from the global registry.
func Unregister(name string) error {
	return globalRegistry.Unregister(name)
}

// GetRegistry returns the global registry instance.
func GetRegistry() *Registry {
	return globalRegistry
}
