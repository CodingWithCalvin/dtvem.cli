package node

import (
	"testing"
	"time"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/lifecycle"
)

func mustParseDate(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func TestLifecycleProvider_VersionStatus(t *testing.T) {
	tests := []struct {
		name    string
		now     string
		version string
		want    string
	}{
		// v22: start 2024-04-24, lts 2024-10-29, maintenance 2025-10-21, end 2027-04-30
		{
			name:    "v22 before LTS is Current",
			now:     "2024-06-15",
			version: "22.3.0",
			want:    string(lifecycle.Current),
		},
		{
			name:    "v22 after LTS date is Active LTS",
			now:     "2025-01-15",
			version: "22.14.0",
			want:    string(lifecycle.ActiveLTS),
		},
		{
			name:    "v22 after maintenance is Maintenance LTS",
			now:     "2026-01-15",
			version: "22.14.0",
			want:    string(lifecycle.MaintenanceLTS),
		},
		{
			name:    "v22 after end is EOL",
			now:     "2027-05-01",
			version: "22.14.0",
			want:    string(lifecycle.EOL),
		},
		// v23: start 2024-10-16, maintenance 2025-04-01, end 2025-06-01 (no LTS)
		{
			name:    "v23 during active period is Current",
			now:     "2025-01-15",
			version: "23.5.0",
			want:    string(lifecycle.Current),
		},
		{
			name:    "v23 odd version in maintenance window is still Current (not LTS)",
			now:     "2025-04-15",
			version: "23.5.0",
			want:    string(lifecycle.Current),
		},
		{
			name:    "v23 after end is EOL",
			now:     "2025-07-01",
			version: "23.5.0",
			want:    string(lifecycle.EOL),
		},
		// v25: start 2025-10-15, maintenance 2026-04-01, end 2026-06-01 (no LTS)
		// Regression for #241: odd version past maintenance but before end
		// should show as Current, not EOL.
		{
			name:    "v25 past maintenance but before end is Current",
			now:     "2026-04-20",
			version: "25.9.0",
			want:    string(lifecycle.Current),
		},
		// Edge cases
		{
			name:    "unknown major returns empty",
			now:     "2025-01-15",
			version: "99.0.0",
			want:    "",
		},
		{
			name:    "invalid version returns empty",
			now:     "2025-01-15",
			version: "abc",
			want:    "",
		},
		{
			name:    "version before start returns empty",
			now:     "2024-04-01",
			version: "22.0.0",
			want:    "",
		},
		{
			name:    "version with v prefix works",
			now:     "2025-01-15",
			version: "v22.14.0",
			want:    string(lifecycle.ActiveLTS),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp := newLifecycleProviderAt(mustParseDate(tt.now))
			got := lp.VersionStatus(tt.version)
			if got != tt.want {
				t.Errorf("VersionStatus(%q) at %s = %q, want %q", tt.version, tt.now, got, tt.want)
			}
		})
	}
}

func TestExtractMajor(t *testing.T) {
	tests := []struct {
		version string
		want    int
	}{
		{"22.14.0", 22},
		{"v22.14.0", 22},
		{"18.16.0", 18},
		{"0.12.0", 0},
		{"abc", -1},
		{"", -1},
	}

	for _, tt := range tests {
		t.Run(tt.version, func(t *testing.T) {
			got := extractMajor(tt.version)
			if got != tt.want {
				t.Errorf("extractMajor(%q) = %d, want %d", tt.version, got, tt.want)
			}
		})
	}
}

func TestParseDate(t *testing.T) {
	tests := []struct {
		input string
		zero  bool
	}{
		{"2024-04-24", false},
		{"", true},
		{"invalid", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseDate(tt.input)
			if got.IsZero() != tt.zero {
				t.Errorf("parseDate(%q).IsZero() = %v, want %v", tt.input, got.IsZero(), tt.zero)
			}
		})
	}
}
