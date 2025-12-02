package runtime

import (
	"fmt"
	"strings"
)

// Version represents a runtime version
type Version struct {
	Raw   string // The raw version string (e.g., "3.11.0", "18.16.0")
	Major int
	Minor int
	Patch int
}

// NewVersion creates a new Version from a version string
func NewVersion(version string) Version {
	return Version{
		Raw: version,
		// TODO: Parse major, minor, patch from version string
		// For now, just store the raw version
	}
}

// String returns the string representation of the version
func (v Version) String() string {
	return v.Raw
}

// Equal checks if two versions are equal
func (v Version) Equal(other Version) bool {
	return v.Raw == other.Raw
}

// InstalledVersion represents an installed runtime version with metadata
type InstalledVersion struct {
	Version
	InstallPath string
	IsGlobal    bool
}

// String returns a formatted string representation
func (iv InstalledVersion) String() string {
	marker := ""
	if iv.IsGlobal {
		marker = " (global)"
	}
	return fmt.Sprintf("%s%s", iv.Version.Raw, marker)
}

// AvailableVersion represents a version available for installation
type AvailableVersion struct {
	Version
	DownloadURL string
	Size        int64
	Checksum    string
	Notes       string // Optional notes (e.g., "LTS", "Latest", "Stable")
}

// DetectedVersion represents a runtime version found on the system
type DetectedVersion struct {
	Version   string // Version string (e.g., "22.0.0", "3.11.0")
	Path      string // Path to the executable
	Source    string // Source of installation (e.g., "system", "nvm", "pyenv")
	Validated bool   // Whether we've verified this version works
}

// String returns a formatted string representation
func (dv DetectedVersion) String() string {
	return fmt.Sprintf("v%s (%s) %s", dv.Version, dv.Source, dv.Path)
}

// ParseVersions converts a slice of version strings to Version objects
func ParseVersions(versions []string) []Version {
	result := make([]Version, len(versions))
	for i, v := range versions {
		result[i] = NewVersion(strings.TrimSpace(v))
	}
	return result
}
