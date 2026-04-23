package main

import "testing"

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
