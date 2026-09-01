package main

import (
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
)

func TestShimNameFromPath(t *testing.T) {
	tests := []struct {
		name     string
		shimPath string
		want     string
	}{
		{
			name:     "unix-style bare binary",
			shimPath: "/home/user/.dtvem/shims/mmdc",
			want:     "mmdc",
		},
		{
			name:     "windows lowercase .exe",
			shimPath: `C:\Users\calvin\.dtvem\shims\mmdc.exe`,
			want:     "mmdc",
		},
		{
			name:     "windows uppercase .EXE (PATHEXT-resolved)",
			shimPath: `C:\Users\calvin\.dtvem\shims\mmdc.EXE`,
			want:     "mmdc",
		},
		{
			name:     "windows mixed case .Exe",
			shimPath: `C:\Users\calvin\.dtvem\shims\mmdc.Exe`,
			want:     "mmdc",
		},
		{
			name:     "forward-slash path with uppercase extension",
			shimPath: "C:/Users/calvin/.dtvem/shims/npm.EXE",
			want:     "npm",
		},
		{
			name:     "bare shim name without extension",
			shimPath: "mmdc",
			want:     "mmdc",
		},
		{
			name:     "non-.exe extension is preserved (not stripped)",
			shimPath: `C:\tools\something.bat`,
			want:     "something.bat",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shimNameFromPath(tt.shimPath)
			if got != tt.want {
				t.Errorf("shimNameFromPath(%q) = %q, want %q", tt.shimPath, got, tt.want)
			}
		})
	}
}

// isolateShimEnv points dtvem at an empty temp root so shim resolution
// runs against the registered providers rather than whatever shim-map
// cache happens to exist on the machine running the tests. Both the
// paths and the shim map memoize on first use, so each is dropped going
// in and coming out.
func isolateShimEnv(t *testing.T) {
	t.Helper()

	tmpRoot := t.TempDir()
	t.Setenv("HOME", tmpRoot)
	t.Setenv("USERPROFILE", tmpRoot)
	t.Setenv("DTVEM_ROOT", tmpRoot)

	config.ResetPathsCache()
	t.Cleanup(config.ResetPathsCache)
	shim.ResetShimMapCache()
	t.Cleanup(shim.ResetShimMapCache)
}

func TestMapShimToRuntime(t *testing.T) {
	isolateShimEnv(t)

	tests := []struct {
		name        string
		shimName    string
		wantRuntime string
		wantKnown   bool
	}{
		{
			name:        "runtime name itself",
			shimName:    "python",
			wantRuntime: "python",
			wantKnown:   true,
		},
		{
			name:        "declared secondary shim",
			shimName:    "pip",
			wantRuntime: "python",
			wantKnown:   true,
		},
		{
			name:        "node package manager",
			shimName:    "npm",
			wantRuntime: "node",
			wantKnown:   true,
		},
		{
			name:        "ruby bundler",
			shimName:    "bundle",
			wantRuntime: "ruby",
			wantKnown:   true,
		},
		{
			name:        "version-suffixed name resolves by prefix",
			shimName:    "python3.13",
			wantRuntime: "python",
			wantKnown:   true,
		},
		// #273: these shims outlived the pip packages that created
		// them. Treating the shim name as a runtime name produced the
		// baffling "runtime provider 'ruff' not found" on invocation;
		// reporting them as unclaimed lets the caller fall through to
		// the real command on PATH.
		{
			name:        "orphaned pip console script",
			shimName:    "ruff",
			wantRuntime: "",
			wantKnown:   false,
		},
		{
			name:        "orphaned test runner",
			shimName:    "pytest",
			wantRuntime: "",
			wantKnown:   false,
		},
		{
			name:        "unknown command",
			shimName:    "definitely-not-a-runtime",
			wantRuntime: "",
			wantKnown:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotRuntime, gotKnown := mapShimToRuntime(tt.shimName)
			if gotRuntime != tt.wantRuntime || gotKnown != tt.wantKnown {
				t.Errorf("mapShimToRuntime(%q) = (%q, %v), want (%q, %v)",
					tt.shimName, gotRuntime, gotKnown, tt.wantRuntime, tt.wantKnown)
			}
		})
	}
}

// TestMapShimToRuntime_CacheClaimsPackageShims covers the healthy path
// for a package executable: no provider's static Shims() list mentions
// `ruff`, so only the shim-map cache written by reshim can route it.
func TestMapShimToRuntime_CacheClaimsPackageShims(t *testing.T) {
	isolateShimEnv(t)

	if err := shim.SaveShimMap(shim.ShimMap{
		"ruff": {Runtime: "python", Versions: []string{"3.13.11"}},
	}); err != nil {
		t.Fatalf("SaveShimMap() error: %v", err)
	}
	shim.ResetShimMapCache()

	gotRuntime, gotKnown := mapShimToRuntime("ruff")
	if gotRuntime != "python" || !gotKnown {
		t.Errorf(`mapShimToRuntime("ruff") = (%q, %v), want ("python", true)`, gotRuntime, gotKnown)
	}
}
