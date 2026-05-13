//go:build shim

package runtime

// Provider is the public type used throughout the codebase for registered
// runtime implementations. In default builds Provider is the full interface
// declared in provider_full.go; in shim builds it collapses to ShimProvider
// so the heavy methods (Install, ListAvailable, etc.) are never referenced
// in the shim binary's import graph.
//
// The registry stores values of type Provider, so this alias keeps the
// registry and its callers (e.g. internal/shim, cmd/shim) unchanged.
type Provider = ShimProvider
