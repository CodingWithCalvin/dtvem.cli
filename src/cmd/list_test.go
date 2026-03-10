package cmd

import (
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/tui"
)

func TestGetVersionStatusWithLifecycle(t *testing.T) {
	tests := []struct {
		name            string
		version         string
		globalVersion   string
		localVersion    string
		lifecycleStatus string
		want            string
	}{
		{
			name:            "lifecycle only",
			version:         "22.14.0",
			globalVersion:   "",
			localVersion:    "",
			lifecycleStatus: "Active LTS",
			want:            tui.RenderLifecycleStatus("Active LTS"),
		},
		{
			name:            "global only",
			version:         "22.14.0",
			globalVersion:   "22.14.0",
			localVersion:    "",
			lifecycleStatus: "",
			want:            globalIndicator + " global",
		},
		{
			name:            "global with lifecycle",
			version:         "22.14.0",
			globalVersion:   "22.14.0",
			localVersion:    "",
			lifecycleStatus: "Active LTS",
			want:            globalIndicator + " global" + " · " + tui.RenderLifecycleStatus("Active LTS"),
		},
		{
			name:            "local with lifecycle",
			version:         "22.14.0",
			globalVersion:   "",
			localVersion:    "22.14.0",
			lifecycleStatus: "Maintenance LTS",
			want:            localIndicator + " local" + " · " + tui.RenderLifecycleStatus("Maintenance LTS"),
		},
		{
			name:            "no status at all",
			version:         "22.14.0",
			globalVersion:   "",
			localVersion:    "",
			lifecycleStatus: "",
			want:            "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getVersionStatusWithLifecycle(tt.version, tt.globalVersion, tt.localVersion, tt.lifecycleStatus)
			if got != tt.want {
				t.Errorf("getVersionStatusWithLifecycle() = %q, want %q", got, tt.want)
			}
		})
	}
}
