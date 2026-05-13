package doctor

import "sync"

// Registry holds an ordered list of registered checks. Registration order
// is preserved so the doctor command produces a stable, predictable
// report (alphabetical/severity sorting happens at render time, not here).
type Registry struct {
	mu     sync.RWMutex
	checks []Check
}

// NewRegistry returns an empty Registry. Most callers should use the
// package-level Register / All instead; NewRegistry is exposed so tests
// can build an isolated registry without touching package state.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register appends a check to the registry. Duplicate names are allowed
// at the type level (the registry is a list, not a map) but discouraged;
// tests should call Reset before registering to keep ordering deterministic.
func (r *Registry) Register(c Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = append(r.checks, c)
}

// All returns the registered checks in registration order. The returned
// slice is a copy, so callers can sort or filter without affecting the
// registry.
func (r *Registry) All() []Check {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Check, len(r.checks))
	copy(out, r.checks)
	return out
}

// Reset clears the registry. Primarily useful in tests that want to
// drive a known set of checks.
func (r *Registry) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = nil
}

var defaultRegistry = NewRegistry()

// Register adds a check to the package-level registry. Checks call this
// from init() so the doctor command discovers them without any explicit
// wiring at startup.
func Register(c Check) {
	defaultRegistry.Register(c)
}

// All returns every check registered in the package-level registry, in
// registration order.
func All() []Check {
	return defaultRegistry.All()
}

// Reset clears the package-level registry. Tests only.
func Reset() {
	defaultRegistry.Reset()
}
