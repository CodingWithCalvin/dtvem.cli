package main

import (
	"encoding/json"
	"strings"
	"testing"
)

const testBaseURL = "https://builds.dtvem.io"

const (
	pbsURL      = "https://github.com/astral-sh/python-build-standalone/releases/download/20250529/cpython-3.12.10%2B20250529-x86_64-pc-windows-msvc-install_only.tar.gz"
	pbsLinuxURL = "https://github.com/astral-sh/python-build-standalone/releases/download/20250529/cpython-3.12.10%2B20250529-x86_64-unknown-linux-gnu-install_only.tar.gz"
)

func meta(sourceURL, sha string) *BinaryMeta {
	return &BinaryMeta{SHA256: sha, SHA256Source: "dtvem", SourceURL: sourceURL}
}

func keys(list ...string) map[string]bool {
	m := make(map[string]bool)
	for _, k := range list {
		m[k] = true
	}
	return m
}

func TestAssembleManifest_Python(t *testing.T) {
	metas := map[string]*BinaryMeta{
		// python-build-standalone default with a python.org alternate.
		"python/3.12.10/windows-amd64.meta.json":            meta(pbsURL, "pbs"),
		"python/3.12.10/linux-amd64.meta.json":              meta(pbsLinuxURL, "linux"),
		"python/3.12.10/python.org/windows-amd64.meta.json": meta("https://www.python.org/ftp/python/3.12.10/amd64/", "org"),
		// The default already is python.org's build.
		"python/3.9.13/windows-amd64.meta.json":            meta("https://www.python.org/ftp/python/3.9.13/amd64/", "org-default"),
		"python/3.9.13/python.org/windows-amd64.meta.json": meta("https://www.python.org/ftp/python/3.9.13/amd64/", "org-alt"),
		// Only a python.org build, from an install-manager zip it repacked.
		"python/3.14.99/python.org/windows-amd64.meta.json": meta("https://www.python.org/ftp/python/3.14.99/python-3.14.99-amd64.zip", "org-only"),
		// A default repacked from an install-manager zip.
		"python/3.15.0/windows-amd64.meta.json": meta("https://www.python.org/ftp/python/3.15.0/python-3.15.0-amd64.zip", "zip-default"),
		// python.org's NuGet package for arm64.
		"python/3.11.4/windows-arm64.meta.json": meta("https://api.nuget.org/v3-flatcontainer/pythonarm64/3.11.4/pythonarm64.3.11.4.nupkg", "nuget"),
		// An embeddable zip, kept as it is.
		"python/3.5.2/windows-amd64.meta.json": meta("https://www.python.org/ftp/python/3.5.2/python-3.5.2-embed-amd64.zip", "embed"),
		// A folder that holds no known build.
		"python/3.12.10/other/windows-amd64.meta.json": meta("https://example.invalid/x.tar.gz", "other"),
		// Another runtime's file.
		"node/22.0.0/linux-amd64.meta.json": meta("https://nodejs.org/dist/v22.0.0/node-v22.0.0-linux-x64.tar.xz", "node"),
	}
	allKeys := keys(
		"python/3.12.10/windows-amd64.tar.gz",
		"python/3.12.10/linux-amd64.tar.gz",
		"python/3.12.10/python.org/windows-amd64.tar.gz",
		"python/3.9.13/windows-amd64.tar.gz",
		"python/3.9.13/windows-amd64.zip",
		"python/3.9.13/python.org/windows-amd64.tar.gz",
		"python/3.14.99/python.org/windows-amd64.tar.gz",
		"python/3.15.0/windows-amd64.tar.gz",
		"python/3.11.4/windows-arm64.tar.gz",
		"python/3.5.2/windows-amd64.zip",
		"python/3.12.10/other/windows-amd64.tar.gz",
	)

	m := assembleManifest("python", testBaseURL, metas, allKeys)

	tests := []struct {
		version, platform string
		wantURL, wantSHA  string
		wantBuild         string
		wantAlternates    []string // URLs
	}{
		{"3.12.10", "windows-amd64", testBaseURL + "/python/3.12.10/windows-amd64.tar.gz", "pbs", buildPythonStandalone,
			[]string{testBaseURL + "/python/3.12.10/python.org/windows-amd64.tar.gz"}},
		{"3.12.10", "linux-amd64", testBaseURL + "/python/3.12.10/linux-amd64.tar.gz", "linux", buildPythonStandalone, nil},
		{"3.9.13", "windows-amd64", testBaseURL + "/python/3.9.13/windows-amd64.tar.gz", "org-default", buildPythonOrg, nil},
		{"3.14.99", "windows-amd64", testBaseURL + "/python/3.14.99/python.org/windows-amd64.tar.gz", "org-only", buildPythonOrg, nil},
		{"3.15.0", "windows-amd64", testBaseURL + "/python/3.15.0/windows-amd64.tar.gz", "zip-default", buildPythonOrg, nil},
		{"3.11.4", "windows-arm64", testBaseURL + "/python/3.11.4/windows-arm64.tar.gz", "nuget", buildPythonOrg, nil},
		{"3.5.2", "windows-amd64", testBaseURL + "/python/3.5.2/windows-amd64.zip", "embed", buildPythonOrg, nil},
	}
	for _, tt := range tests {
		t.Run(tt.version+"/"+tt.platform, func(t *testing.T) {
			d := m.Versions[tt.version][tt.platform]
			if d == nil {
				t.Fatalf("no entry for %s %s", tt.version, tt.platform)
			}
			if d.URL != tt.wantURL || d.SHA256 != tt.wantSHA || d.Build != tt.wantBuild {
				t.Errorf("entry = {%s %s %s}, want {%s %s %s}", d.URL, d.SHA256, d.Build, tt.wantURL, tt.wantSHA, tt.wantBuild)
			}
			var alts []string
			for _, a := range d.Alternates {
				alts = append(alts, a.URL)
				if a.Build != buildPythonOrg || len(a.Alternates) != 0 {
					t.Errorf("alternate %s: build %q with %d alternates, want python.org with none", a.URL, a.Build, len(a.Alternates))
				}
			}
			if strings.Join(alts, ",") != strings.Join(tt.wantAlternates, ",") {
				t.Errorf("alternates = %v, want %v", alts, tt.wantAlternates)
			}
		})
	}

	if alt := m.Versions["3.12.10"]["windows-amd64"].Alternates[0]; alt.SHA256 != "org" || alt.SHA256Source != "dtvem" {
		t.Errorf("3.12.10 python.org alternate = {%s %s}, want {org dtvem}", alt.SHA256, alt.SHA256Source)
	}
	if _, ok := m.Versions["22.0.0"]; ok {
		t.Error("python manifest includes node's 22.0.0")
	}
	if n := len(m.Versions["3.12.10"]); n != 2 {
		t.Errorf("3.12.10 has %d platforms, want 2", n)
	}
}

// Runtimes with one build get no build labels, so their manifests are
// unchanged.
func TestAssembleManifest_SingleBuildRuntimes(t *testing.T) {
	metas := map[string]*BinaryMeta{
		"node/22.0.0/linux-amd64.meta.json":   meta("https://nodejs.org/dist/v22.0.0/node-v22.0.0-linux-x64.tar.xz", "node"),
		"node/22.0.0/windows-amd64.meta.json": meta("https://nodejs.org/dist/v22.0.0/node-v22.0.0-win-x64.zip", "node-win"),
		// The source URL's extension wins when R2 has it, as before.
		"node/22.0.0/darwin-arm64.meta.json": meta("https://nodejs.org/dist/v22.0.0/node-v22.0.0-darwin-arm64.tar.gz", "node-mac"),
	}
	allKeys := keys(
		"node/22.0.0/linux-amd64.tar.xz",
		"node/22.0.0/windows-amd64.zip",
		"node/22.0.0/darwin-arm64.tar.gz",
		"node/22.0.0/darwin-arm64.zip",
	)
	m := assembleManifest("node", testBaseURL, metas, allKeys)

	for platform, want := range map[string]string{
		"linux-amd64":   testBaseURL + "/node/22.0.0/linux-amd64.tar.xz",
		"windows-amd64": testBaseURL + "/node/22.0.0/windows-amd64.zip",
		"darwin-arm64":  testBaseURL + "/node/22.0.0/darwin-arm64.tar.gz",
	} {
		d := m.Versions["22.0.0"][platform]
		if d == nil || d.URL != want || d.Build != "" {
			t.Errorf("node 22.0.0 %s = %+v, want URL %s and no build", platform, d, want)
		}
	}

	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"build"`) || strings.Contains(string(data), `"alternates"`) {
		t.Errorf("node manifest has build fields: %s", data)
	}

	rubyMetas := map[string]*BinaryMeta{
		"ruby/3.4.1/linux-amd64.meta.json": meta("built-from-source", "ruby"),
	}
	rm := assembleManifest("ruby", testBaseURL, rubyMetas, keys("ruby/3.4.1/linux-amd64.tar.gz"))
	if d := rm.Versions["3.4.1"]["linux-amd64"]; d == nil || d.Source != "built-from-source" || d.URL != testBaseURL+"/ruby/3.4.1/linux-amd64.tar.gz" {
		t.Errorf("ruby 3.4.1 = %+v, want a built-from-source .tar.gz", d)
	}
}

func TestAssembleManifest_MissingBinary(t *testing.T) {
	metas := map[string]*BinaryMeta{
		// Neither the source URL nor R2 tells the extension: skipped.
		"python/3.13.1/python.org/windows-amd64.meta.json": meta("https://www.python.org/ftp/python/3.13.1/amd64/", "x"),
		// The source URL's extension is used even when R2 lacks it, as
		// before.
		"python/3.13.1/linux-amd64.meta.json": meta(pbsLinuxURL, "y"),
	}
	m := assembleManifest("python", testBaseURL, metas, keys())

	if d := m.Versions["3.13.1"]["windows-amd64"]; d != nil {
		t.Errorf("windows-amd64 = %+v, want no entry without a binary", d)
	}
	if d := m.Versions["3.13.1"]["linux-amd64"]; d == nil || !strings.HasSuffix(d.URL, "/python/3.13.1/linux-amd64.tar.gz") {
		t.Errorf("linux-amd64 = %+v, want the source URL's extension", d)
	}
}

func TestBuildFromSourceURL(t *testing.T) {
	tests := []struct {
		runtime, url, want string
	}{
		{"python", pbsURL, buildPythonStandalone},
		{"python", "https://github.com/indygreg/python-build-standalone/releases/download/20240107/cpython-3.8.18%2B20240107-x86_64-pc-windows-msvc-shared-install_only.tar.gz", buildPythonStandalone},
		{"python", "https://www.python.org/ftp/python/3.9.13/amd64/", buildPythonOrg},
		{"python", "https://www.python.org/ftp/python/3.15.2/python-3.15.2-amd64.zip", buildPythonOrg},
		{"python", "https://api.nuget.org/v3-flatcontainer/pythonarm64/3.11.4/pythonarm64.3.11.4.nupkg", buildPythonOrg},
		{"python", "built-from-source", ""},
		{"python", "https://example.invalid/python.tar.gz", ""},
		{"node", "https://www.python.org/ftp/python/3.9.13/amd64/", ""},
	}
	for _, tt := range tests {
		if got := buildFromSourceURL(tt.runtime, tt.url); got != tt.want {
			t.Errorf("buildFromSourceURL(%s, %s) = %q, want %q", tt.runtime, tt.url, got, tt.want)
		}
	}
}

// A release python.org ships before python-build-standalone does is listed
// with python.org's build as its default; once python-build-standalone's
// build is mirrored, that becomes the default and python.org's the
// alternate.
func TestAssembleManifest_DefaultArrivesLater(t *testing.T) {
	org := meta("https://www.python.org/ftp/python/3.14.11/python-3.14.11-amd64.zip", "org")
	before := assembleManifest("python", testBaseURL,
		map[string]*BinaryMeta{"python/3.14.11/python.org/windows-amd64.meta.json": org},
		keys("python/3.14.11/python.org/windows-amd64.tar.gz"))

	d := before.Versions["3.14.11"]["windows-amd64"]
	if d == nil || d.Build != buildPythonOrg || d.URL != testBaseURL+"/python/3.14.11/python.org/windows-amd64.tar.gz" || len(d.Alternates) != 0 {
		t.Fatalf("before python-build-standalone: %+v, want python.org's build as the default", d)
	}

	after := assembleManifest("python", testBaseURL,
		map[string]*BinaryMeta{
			"python/3.14.11/python.org/windows-amd64.meta.json": org,
			"python/3.14.11/windows-amd64.meta.json":            meta("https://github.com/astral-sh/python-build-standalone/releases/download/20261120/cpython-3.14.11%2B20261120-x86_64-pc-windows-msvc-install_only.tar.gz", "pbs"),
		},
		keys("python/3.14.11/python.org/windows-amd64.tar.gz", "python/3.14.11/windows-amd64.tar.gz"))

	d = after.Versions["3.14.11"]["windows-amd64"]
	if d == nil || d.Build != buildPythonStandalone || d.URL != testBaseURL+"/python/3.14.11/windows-amd64.tar.gz" {
		t.Fatalf("after python-build-standalone: %+v, want its build as the default", d)
	}
	if len(d.Alternates) != 1 || d.Alternates[0].Build != buildPythonOrg || d.Alternates[0].SHA256 != "org" {
		t.Errorf("after python-build-standalone: alternates %+v, want python.org's build", d.Alternates)
	}
}
