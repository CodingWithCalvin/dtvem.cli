package manifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// buildsManifestJSON has a python-build-standalone default with a
// python.org alternate (3.12.10), a python.org default (3.9.13), a version
// with no python.org build (3.12.12) and an entry from before build labels
// (3.11.0).
const buildsManifestJSON = `{
  "version": 1,
  "versions": {
    "3.12.10": {
      "windows-amd64": {
        "url": "https://builds.dtvem.io/python/3.12.10/windows-amd64.tar.gz",
        "sha256": "3e0fd10e4bd60e66baad8be2a0e60775bb389e97e4f4a07138c34599c356aaed",
        "sha256_source": "dtvem",
        "build": "python-build-standalone",
        "alternates": [
          {
            "url": "https://builds.dtvem.io/python/3.12.10/python.org/windows-amd64.tar.gz",
            "sha256": "1111111111111111111111111111111111111111111111111111111111111111",
            "sha256_source": "dtvem",
            "build": "python.org"
          }
        ]
      },
      "linux-amd64": {
        "url": "https://builds.dtvem.io/python/3.12.10/linux-amd64.tar.gz",
        "build": "python-build-standalone"
      }
    },
    "3.12.12": {
      "windows-amd64": {
        "url": "https://builds.dtvem.io/python/3.12.12/windows-amd64.tar.gz",
        "sha256": "33a611265c1abfd245ea1fd23273f8535857883552be1bd8b62dfe7db9ef0694",
        "sha256_source": "upstream",
        "build": "python-build-standalone"
      }
    },
    "3.9.13": {
      "windows-amd64": {
        "url": "https://builds.dtvem.io/python/3.9.13/windows-amd64.tar.gz",
        "sha256": "34981d11032b07867ad4af3c9a501aafb9602a8526abdfd652423c9edc491684",
        "sha256_source": "dtvem",
        "build": "python.org"
      },
      "windows-arm64": null
    },
    "3.11.0": {
      "windows-amd64": {
        "url": "https://builds.dtvem.io/python/3.11.0/windows-amd64.tar.gz"
      }
    }
  }
}`

func TestDownloadForBuild(t *testing.T) {
	m, err := ParseManifest([]byte(buildsManifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error: %v", err)
	}

	tests := []struct {
		name              string
		version, platform string
		build             string
		wantURL           string
	}{
		{"alternate", "3.12.10", "windows-amd64", BuildPythonOrg, "https://builds.dtvem.io/python/3.12.10/python.org/windows-amd64.tar.gz"},
		{"default is the build", "3.12.10", "windows-amd64", BuildPythonStandalone, "https://builds.dtvem.io/python/3.12.10/windows-amd64.tar.gz"},
		{"python.org default", "3.9.13", "windows-amd64", BuildPythonOrg, "https://builds.dtvem.io/python/3.9.13/windows-amd64.tar.gz"},
		{"no such build", "3.12.12", "windows-amd64", BuildPythonOrg, ""},
		{"unlabeled entry", "3.11.0", "windows-amd64", BuildPythonOrg, ""},
		{"no build asked for", "3.12.10", "windows-amd64", "", ""},
		{"unavailable platform", "3.9.13", "windows-arm64", BuildPythonOrg, ""},
		{"missing platform", "3.12.12", "linux-amd64", BuildPythonOrg, ""},
		{"missing version", "3.99.0", "windows-amd64", BuildPythonOrg, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.GetDownload(tt.version, tt.platform).ForBuild(tt.build)
			gotURL := ""
			if got != nil {
				gotURL = got.URL
			}
			if gotURL != tt.wantURL {
				t.Errorf("ForBuild(%q) URL = %q, want %q", tt.build, gotURL, tt.wantURL)
			}
		})
	}
}

func TestManifestListBuildVersions(t *testing.T) {
	m, err := ParseManifest([]byte(buildsManifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error: %v", err)
	}

	got := m.ListBuildVersions("windows-amd64", BuildPythonOrg)
	sort.Strings(got)
	if want := "3.12.10,3.9.13"; strings.Join(got, ",") != want {
		t.Errorf("ListBuildVersions(windows-amd64, python.org) = %v, want %s", got, want)
	}
	if got := m.ListBuildVersions("linux-amd64", BuildPythonOrg); len(got) != 0 {
		t.Errorf("ListBuildVersions(linux-amd64, python.org) = %v, want none", got)
	}
}

// legacyDownload is Download as CLIs released before builds and alternates
// read it.
type legacyDownload struct {
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	SHA256Source string `json:"sha256_source,omitempty"`
}

// A manifest with builds and alternates still gives released CLIs the same
// default download as before.
func TestBuildsManifest_LegacyCLI(t *testing.T) {
	var legacy struct {
		Version  int                                   `json:"version"`
		Versions map[string]map[string]*legacyDownload `json:"versions"`
	}
	if err := json.Unmarshal([]byte(buildsManifestJSON), &legacy); err != nil {
		t.Fatalf("legacy parse error: %v", err)
	}
	got := legacy.Versions["3.12.10"]["windows-amd64"]
	want := legacyDownload{
		URL:          "https://builds.dtvem.io/python/3.12.10/windows-amd64.tar.gz",
		SHA256:       "3e0fd10e4bd60e66baad8be2a0e60775bb389e97e4f4a07138c34599c356aaed",
		SHA256Source: "dtvem",
	}
	if got == nil || *got != want {
		t.Errorf("legacy 3.12.10 windows-amd64 = %+v, want %+v", got, want)
	}
}

func TestCachedSource_KeepsBuilds(t *testing.T) {
	m, err := ParseManifest([]byte(buildsManifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error: %v", err)
	}
	mock := newMockSource()
	mock.manifests["python"] = m
	source := NewCachedSource(mock, t.TempDir(), time.Hour)

	if _, err := source.GetManifest("python"); err != nil {
		t.Fatalf("first GetManifest() error: %v", err)
	}
	cached, err := source.GetManifest("python")
	if err != nil {
		t.Fatalf("second GetManifest() error: %v", err)
	}
	if mock.callCount["python"] != 1 {
		t.Errorf("source called %d times, want 1 (second call from cache)", mock.callCount["python"])
	}
	if cached.GetDownload("3.12.10", "windows-amd64").ForBuild(BuildPythonOrg) == nil {
		t.Error("cached manifest lost the python.org alternate")
	}
}

// A cache file written by a CLI that did not know about builds holds the
// manifest without them, so it is fetched again rather than used.
func TestCachedSource_IgnoresOlderFormat(t *testing.T) {
	dir := t.TempDir()
	old := `{"cached_at":"` + time.Now().Format(time.RFC3339Nano) + `","manifest":{"version":1,"versions":{"3.12.10":{"windows-amd64":{"url":"https://builds.dtvem.io/python/3.12.10/windows-amd64.tar.gz","sha256":"x"}}}}}`
	if err := os.WriteFile(filepath.Join(dir, "python.cache.json"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}

	m, err := ParseManifest([]byte(buildsManifestJSON))
	if err != nil {
		t.Fatalf("ParseManifest() error: %v", err)
	}
	mock := newMockSource()
	mock.manifests["python"] = m
	source := NewCachedSource(mock, dir, time.Hour)

	got, err := source.GetManifest("python")
	if err != nil {
		t.Fatalf("GetManifest() error: %v", err)
	}
	if mock.callCount["python"] != 1 {
		t.Errorf("source called %d times, want 1 (older-format cache ignored)", mock.callCount["python"])
	}
	if got.GetDownload("3.12.10", "windows-amd64").ForBuild(BuildPythonOrg) == nil {
		t.Error("manifest has no python.org alternate; the older-format cache was used")
	}
}

// A newly installed dtvem fetches the manifests again instead of reading
// ones another version cached, even in the current format. A pre-release
// build may have cached a manifest from before the release's fields existed.
func TestCachedSource_IgnoresOtherVersion(t *testing.T) {
	tests := []struct {
		name          string
		writer        string // version that cached the manifest
		reader        string // version reading it
		wantFetches   int
		wantAlternate bool
	}{
		{name: "pre-release cache, new release", writer: "dev", reader: "0.7.4", wantFetches: 1, wantAlternate: true},
		{name: "older release cache, new release", writer: "0.7.3", reader: "0.7.4", wantFetches: 1, wantAlternate: true},
		{name: "cache from no recorded version, new release", writer: "", reader: "0.7.4", wantFetches: 1, wantAlternate: true},
		{name: "same version reads its own cache", writer: "0.7.4", reader: "0.7.4", wantFetches: 0, wantAlternate: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			// The cached manifest predates the python.org builds
			stale, err := ParseManifest([]byte(`{"version":1,"versions":{"3.12.10":{"windows-amd64":{"url":"https://builds.dtvem.io/python/3.12.10/windows-amd64.tar.gz","sha256":"x"}}}}`))
			if err != nil {
				t.Fatalf("ParseManifest(stale) error: %v", err)
			}
			writer := NewCachedSource(newMockSource(), dir, time.Hour)
			writer.version = tt.writer
			if err := writer.saveToCache("python", stale); err != nil {
				t.Fatalf("saveToCache() error: %v", err)
			}

			fresh, err := ParseManifest([]byte(buildsManifestJSON))
			if err != nil {
				t.Fatalf("ParseManifest(fresh) error: %v", err)
			}
			mock := newMockSource()
			mock.manifests["python"] = fresh
			reader := NewCachedSource(mock, dir, time.Hour)
			reader.version = tt.reader

			got, err := reader.GetManifest("python")
			if err != nil {
				t.Fatalf("GetManifest() error: %v", err)
			}
			if mock.callCount["python"] != tt.wantFetches {
				t.Errorf("source called %d times, want %d", mock.callCount["python"], tt.wantFetches)
			}
			hasAlternate := got.GetDownload("3.12.10", "windows-amd64").ForBuild(BuildPythonOrg) != nil
			if hasAlternate != tt.wantAlternate {
				t.Errorf("python.org alternate present = %v, want %v", hasAlternate, tt.wantAlternate)
			}

			// The refetched manifest is cached for the reading version
			if tt.wantFetches > 0 {
				if _, err := reader.GetManifest("python"); err != nil {
					t.Fatalf("second GetManifest() error: %v", err)
				}
				if mock.callCount["python"] != tt.wantFetches {
					t.Errorf("second call fetched again: source called %d times, want %d", mock.callCount["python"], tt.wantFetches)
				}
			}
		})
	}
}

func TestSetCLIVersion_DefaultSource(t *testing.T) {
	t.Cleanup(func() {
		SetCLIVersion("")
		ResetDefaultSource()
	})

	ResetDefaultSource()
	SetCLIVersion("0.7.4")
	DefaultSource()

	if defaultCached == nil {
		t.Fatal("DefaultSource() did not create the cached source")
	}
	if defaultCached.version != "0.7.4" {
		t.Errorf("default cache version = %q, want %q", defaultCached.version, "0.7.4")
	}
}
