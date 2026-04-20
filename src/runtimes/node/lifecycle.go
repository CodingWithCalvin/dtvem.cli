package node

import (
	"embed"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/lifecycle"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/ui"
)

//go:embed data/schedule.json
var scheduleData embed.FS

// scheduleEntry represents a single major-version entry from the Node.js
// release schedule (https://github.com/nodejs/Release/blob/main/schedule.json).
type scheduleEntry struct {
	Start       string `json:"start"`
	LTS         string `json:"lts,omitempty"`
	Maintenance string `json:"maintenance,omitempty"`
	End         string `json:"end"`
	Codename    string `json:"codename,omitempty"`
}

// lifecycleProvider computes Node.js lifecycle status from the embedded
// release schedule. It implements lifecycle.StatusProvider.
type lifecycleProvider struct {
	schedule map[string]scheduleEntry
	now      time.Time
}

// Ensure lifecycleProvider satisfies the interface at compile time.
var _ lifecycle.StatusProvider = (*lifecycleProvider)(nil)

// newLifecycleProvider parses the embedded schedule and returns a provider
// that resolves lifecycle status relative to the current date.
func newLifecycleProvider() *lifecycleProvider {
	return newLifecycleProviderAt(time.Now())
}

// newLifecycleProviderAt is a test-friendly variant that accepts an explicit
// reference date instead of using the wall clock.
func newLifecycleProviderAt(now time.Time) *lifecycleProvider {
	data, err := scheduleData.ReadFile("data/schedule.json")
	if err != nil {
		ui.Debug("lifecycle: failed to read embedded schedule: %v", err)
		return &lifecycleProvider{now: now}
	}

	var schedule map[string]scheduleEntry
	if err := json.Unmarshal(data, &schedule); err != nil {
		ui.Debug("lifecycle: failed to parse schedule: %v", err)
		return &lifecycleProvider{now: now}
	}

	return &lifecycleProvider{schedule: schedule, now: now}
}

// VersionStatus returns the lifecycle label for a Node.js version string
// (e.g., "22.14.0" → "Active LTS"). Returns "" if unknown.
func (lp *lifecycleProvider) VersionStatus(version string) string {
	major := extractMajor(version)
	if major < 0 {
		return ""
	}

	key := "v" + strconv.Itoa(major)
	entry, ok := lp.schedule[key]
	if !ok {
		return ""
	}

	return lp.resolve(entry)
}

// resolve determines the lifecycle status of a schedule entry relative to lp.now.
func (lp *lifecycleProvider) resolve(e scheduleEntry) string {
	today := lp.now

	end := parseDate(e.End)
	if end.IsZero() {
		return ""
	}
	if !today.Before(end) {
		return string(lifecycle.EOL)
	}

	if maint := parseDate(e.Maintenance); !maint.IsZero() && !today.Before(maint) {
		if e.LTS != "" {
			return string(lifecycle.MaintenanceLTS)
		}
		// Odd releases have a maintenance window before EOL but are not LTS;
		// keep them labeled Current until their end date so the display
		// matches nodejs.org's release schedule page.
		return string(lifecycle.Current)
	}

	if lts := parseDate(e.LTS); !lts.IsZero() && !today.Before(lts) {
		return string(lifecycle.ActiveLTS)
	}

	start := parseDate(e.Start)
	if !start.IsZero() && !today.Before(start) {
		return string(lifecycle.Current)
	}

	return ""
}

// extractMajor returns the major version from a version string like "22.14.0".
// Returns -1 if parsing fails.
func extractMajor(version string) int {
	version = strings.TrimPrefix(version, "v")
	parts := strings.SplitN(version, ".", 2)
	if len(parts) == 0 {
		return -1
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1
	}
	return major
}

// parseDate parses a "YYYY-MM-DD" date string. Returns zero time on failure.
func parseDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}
	}
	return t
}
