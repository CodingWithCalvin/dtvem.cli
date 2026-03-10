package runtime

import (
	"fmt"
	"sort"
	"strconv"
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
	DownloadURL     string
	Size            int64
	Checksum        string
	LifecycleStatus string // Optional lifecycle label (e.g., "Active LTS", "EOL")
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

// SortVersionsDesc sorts AvailableVersions by semantic version in descending order (newest first).
func SortVersionsDesc(versions []AvailableVersion) {
	sort.Slice(versions, func(i, j int) bool {
		return compareVersionStrings(versions[i].Version.Raw, versions[j].Version.Raw) > 0
	})
}

// compareVersionStrings compares two version strings semantically.
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersionStrings(a, b string) int {
	aParts := parseVersionParts(a)
	bParts := parseVersionParts(b)

	// Compare each part
	maxLen := len(aParts)
	if len(bParts) > maxLen {
		maxLen = len(bParts)
	}

	for i := 0; i < maxLen; i++ {
		var aVal, bVal int
		if i < len(aParts) {
			aVal = aParts[i]
		}
		if i < len(bParts) {
			bVal = bParts[i]
		}

		if aVal != bVal {
			return aVal - bVal
		}
	}

	return 0
}

// parseVersionParts splits a version string into numeric parts.
// For example, "3.11.0" becomes [3, 11, 0].
func parseVersionParts(version string) []int {
	// Remove common prefixes
	version = strings.TrimPrefix(version, "v")

	// Split by dots and dashes
	parts := strings.FieldsFunc(version, func(c rune) bool {
		return c == '.' || c == '-'
	})

	var result []int
	for _, part := range parts {
		// Try to parse as integer, skip non-numeric parts
		if val, err := strconv.Atoi(part); err == nil {
			result = append(result, val)
		}
	}

	return result
}
