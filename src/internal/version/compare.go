package version

import (
	"strings"
)

// Compare compares two version strings semantically. It returns a positive
// number if a > b, a negative one if a < b, and 0 if they are equal. A
// pre-release sorts below the release it precedes.
func Compare(a, b string) int {
	return compareVersionStrings(a, b)
}

// MajorMinor returns the major.minor line of a version ("3.12" for
// "3.12.10"), or the version itself when it has fewer parts.
func MajorMinor(v string) string {
	v = strings.TrimPrefix(v, "v")
	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}

// Nearest returns the version in candidates nearest to target on target's
// major.minor line: target itself if present, else the highest candidate
// below it, else the lowest above it. It returns "" when no candidate is on
// that line.
func Nearest(target string, candidates []string) string {
	line := MajorMinor(target)
	var below, above string
	for _, c := range candidates {
		if MajorMinor(c) != line {
			continue
		}
		switch cmp := Compare(c, target); {
		case cmp == 0:
			return c
		case cmp < 0:
			if below == "" || Compare(c, below) > 0 {
				below = c
			}
		default:
			if above == "" || Compare(c, above) < 0 {
				above = c
			}
		}
	}
	if below != "" {
		return below
	}
	return above
}
