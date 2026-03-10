// Package lifecycle provides types for runtime version lifecycle status.
// Providers can implement StatusProvider to surface lifecycle information
// (e.g., LTS, EOL) in version listings.
package lifecycle

// Status represents a version's position in its runtime's release lifecycle.
type Status string

const (
	// Current indicates an actively developed release line (typically the latest).
	Current Status = "Current"

	// ActiveLTS indicates a release receiving active long-term support.
	ActiveLTS Status = "Active LTS"

	// MaintenanceLTS indicates a release receiving only critical fixes.
	MaintenanceLTS Status = "Maintenance LTS"

	// EOL indicates a release that has reached end of life.
	EOL Status = "EOL"
)

// StatusProvider returns lifecycle status for a given version string.
// Providers that track release schedules (e.g., Node.js) can implement
// this interface so list-all displays lifecycle information.
type StatusProvider interface {
	// VersionStatus returns the lifecycle status label for the given version,
	// or an empty string if the status is unknown.
	VersionStatus(version string) string
}
