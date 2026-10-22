package cmd

import (
	"slices"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

func TestInstallVersion(t *testing.T) {
	available := []runtime.AvailableVersion{
		makeAvailableVersion("22.15.1"),
		makeAvailableVersion("22.15.0"),
		makeAvailableVersion("22.0.0"),
		makeAvailableVersion("20.18.0"),
	}

	tests := []struct {
		name    string
		version string
		want    []string
	}{
		{name: "exact version installs as is", version: "22.0.0", want: []string{"22.0.0"}},
		{name: "partial version installs latest match", version: "22", want: []string{"22.15.1"}},
		{name: "partial major.minor installs latest patch", version: "22.15", want: []string{"22.15.1"}},
		{name: "partial with no match installs nothing", version: "21", want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provider := &mockProvider{
				name:              "node",
				displayName:       "Node.js",
				availableVersions: available,
			}

			installVersion(provider, tt.version)

			if !slices.Equal(provider.installCalls, tt.want) {
				t.Errorf("installVersion(%q) installed %v, want %v", tt.version, provider.installCalls, tt.want)
			}
		})
	}
}
