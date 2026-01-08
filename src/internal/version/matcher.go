// Package version provides utilities for resolving partial version specifications
// to full semantic versions from a list of available versions.
package version

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// ResolvePartialVersion finds the latest version matching a partial input.
// Examples:
//   - "22" matches "22.0.0", "22.15.0" → returns highest "22.x.x"
//   - "14.21" matches "14.21.0", "14.21.3" → returns highest "14.21.x"
//   - "22.0.0" with 3 components → returns "22.0.0" (exact match expected)
//
// Returns an error if no matching version is found.
func ResolvePartialVersion(input string, available []string) (string, error) {
	input = strings.TrimPrefix(input, "v")

	// Parse input into components
	inputParts := strings.Split(input, ".")

	// If it's a full semver (3 components), return as-is without validation
	// The caller is responsible for checking if this exact version exists
	if len(inputParts) >= 3 {
		return input, nil
	}

	// Find all versions that match the partial specification
	var matches []string
	for _, v := range available {
		if matchesPartial(v, inputParts) {
			matches = append(matches, v)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no version matching %q found", input)
	}

	// Sort matches by semantic version (descending) and return the highest
	sortVersionsDesc(matches)
	return matches[0], nil
}

// IsPartialVersion returns true if the input has fewer than 3 components.
// Examples:
//   - "22" → true (1 component)
//   - "22.15" → true (2 components)
//   - "22.15.0" → false (3 components)
//   - "v22" → true (1 component, after stripping v prefix)
func IsPartialVersion(input string) bool {
	input = strings.TrimPrefix(input, "v")
	parts := strings.Split(input, ".")
	return len(parts) < 3
}

// matchesPartial checks if a version matches the partial specification.
// The version must have the same numeric values in the positions specified.
func matchesPartial(version string, partialParts []string) bool {
	version = strings.TrimPrefix(version, "v")
	versionParts := strings.Split(version, ".")

	// Version must have at least as many parts as the partial
	if len(versionParts) < len(partialParts) {
		return false
	}

	// Check each partial component matches
	for i, partial := range partialParts {
		// Parse both as integers for numeric comparison
		partialNum, partialErr := strconv.Atoi(partial)
		versionNum, versionErr := strconv.Atoi(versionParts[i])

		if partialErr != nil || versionErr != nil {
			// Fall back to string comparison if not numeric
			if partial != versionParts[i] {
				return false
			}
		} else if partialNum != versionNum {
			return false
		}
	}

	return true
}

// sortVersionsDesc sorts version strings by semantic version in descending order.
func sortVersionsDesc(versions []string) {
	sort.Slice(versions, func(i, j int) bool {
		return compareVersionStrings(versions[i], versions[j]) > 0
	})
}

// compareVersionStrings compares two version strings semantically.
// Returns >0 if a > b, <0 if a < b, 0 if equal.
func compareVersionStrings(a, b string) int {
	aParts := parseVersionParts(a)
	bParts := parseVersionParts(b)

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
func parseVersionParts(version string) []int {
	version = strings.TrimPrefix(version, "v")

	parts := strings.FieldsFunc(version, func(c rune) bool {
		return c == '.' || c == '-'
	})

	var result []int
	for _, part := range parts {
		if val, err := strconv.Atoi(part); err == nil {
			result = append(result, val)
		}
	}

	return result
}
