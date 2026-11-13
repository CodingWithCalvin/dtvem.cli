package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

// installManagerZipEntries is a full install as the install-manager zips
// lay it out: everything at the root of the zip.
func installManagerZipEntries() map[string]string {
	return map[string]string{
		"__install__.json":            "{}",
		"python.exe":                  "exe",
		"python315.dll":               "dll",
		"Lib/os.py":                   "os",
		"Lib/venv/__init__.py":        "venv",
		"Lib/ensurepip/__init__.py":   "ensurepip",
		"Lib/script%20(dev).tmpl":     "not escaped",
		"include/Python.h":            "header",
		"libs/python315.lib":          "lib",
		"DLLs/_tkinter.pyd":           "tkinter",
		"Lib/tkinter/__init__.py":     "tkinter",
		"Lib/site-packages/README.md": "readme",
	}
}

// indexEntry is one install-manager index entry.
func indexEntry(id, version, url, sha string) map[string]any {
	return map[string]any{
		"id":           id,
		"sort-version": version,
		"url":          url,
		"hash":         map[string]string{"sha256": sha},
	}
}

// installManagerServer serves index pages at /<name> and files at
// /ftp/<name>.
func installManagerServer(t *testing.T, pages map[string]map[string]any, files map[string][]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if name, ok := strings.CutPrefix(r.URL.Path, "/ftp/"); ok {
			if body, ok := files[name]; ok {
				_, _ = w.Write(body)
				return
			}
		}
		if page, ok := pages[strings.TrimPrefix(r.URL.Path, "/")]; ok {
			_ = json.NewEncoder(w).Encode(page)
			return
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestPythonInstallManagerSource_FetchVersions(t *testing.T) {
	var srv *httptest.Server
	good := strings.Repeat("ab", 32)
	ftp := func(name string) string { return srv.URL + "/ftp/" + name }
	pages := map[string]map[string]any{}
	srv = installManagerServer(t, pages, nil)

	pages["index-windows.json"] = map[string]any{
		"versions": []any{
			indexEntry("pythoncore-3.15-64", "3.15.2", ftp("python-3.15.2-amd64.zip"), good),
			// Other architectures, free-threaded builds and pre-releases.
			indexEntry("pythoncore-3.15-arm64", "3.15.2", ftp("python-3.15.2-arm64.zip"), good),
			indexEntry("pythoncore-3.15t-64", "3.15.2", ftp("python-3.15.2t-amd64.zip"), good),
			indexEntry("pythoncore-3.15-32", "3.15.2", ftp("python-3.15.2-win32.zip"), good),
			indexEntry("pythoncore-3.16-64", "3.16.0a2", ftp("python-3.16.0a2-amd64.zip"), good),
			// From elsewhere, without a usable hash, or not a zip.
			indexEntry("pythoncore-3.14-64", "3.14.9", "https://example.invalid/python-3.14.9-amd64.zip", good),
			indexEntry("pythoncore-3.14-64", "3.14.8", ftp("python-3.14.8-amd64.zip"), "not-a-hash"),
			indexEntry("pythoncore-3.14-64", "3.14.7", ftp("python-3.14.7-amd64.nupkg"), good),
			// An id that disagrees with its version.
			indexEntry("pythoncore-3.13-64", "3.14.6", ftp("python-3.14.6-amd64.zip"), good),
		},
		"next": "index-windows-recent.json",
	}
	pages["index-windows-recent.json"] = map[string]any{
		"versions": []any{
			indexEntry("pythoncore-3.14-64", "3.14.0", ftp("python-3.14.0-amd64.zip"), strings.ToUpper(good)),
			// Covered by the MSIs and older than this source's range.
			indexEntry("pythoncore-3.13-64", "3.13.9", ftp("python-3.13.9-amd64.zip"), good),
			// Listed again on a later page.
			indexEntry("pythoncore-3.15-64", "3.15.2", ftp("python-3.15.2-amd64-dup.zip"), good),
		},
	}

	src := &PythonInstallManagerSource{indexURL: srv.URL + "/index-windows.json", downloadPrefix: srv.URL + "/ftp/"}
	jobs, err := src.FetchVersions()
	if err != nil {
		t.Fatalf("FetchVersions() error: %v", err)
	}

	versions := checkPythonOrgJobPairs(t, jobs)
	if want := []string{"3.15.2", "3.14.0"}; !reflect.DeepEqual(versions, want) {
		t.Errorf("FetchVersions() versions = %v, want %v", versions, want)
	}
	for _, j := range jobs {
		if j.Platform != "windows-amd64" {
			t.Errorf("job %s: platform %q, want windows-amd64", j.Version, j.Platform)
		}
	}
	if want := ftp("python-3.15.2-amd64.zip"); jobs[0].URL != want {
		t.Errorf("3.15.2 job URL = %q, want %q", jobs[0].URL, want)
	}
}

func TestPythonInstallManagerSource_IndexErrors(t *testing.T) {
	pages := map[string]map[string]any{
		"loop.json": {"versions": []any{}, "next": "loop.json"},
	}
	srv := installManagerServer(t, pages, nil)

	for name, indexURL := range map[string]string{
		"page links to itself": srv.URL + "/loop.json",
		"missing page":         srv.URL + "/missing.json",
	} {
		src := &PythonInstallManagerSource{indexURL: indexURL, downloadPrefix: srv.URL + "/ftp/"}
		if _, err := src.FetchVersions(); err == nil {
			t.Errorf("%s: FetchVersions() = nil error, want error", name)
		}
	}
}

func TestBuildFromInstallManagerZip(t *testing.T) {
	full := nupkg(t, installManagerZipEntries())
	embeddableEntries := installManagerZipEntries()
	embeddableEntries["python315._pth"] = "python315.zip"
	embeddable := nupkg(t, embeddableEntries)

	srv := installManagerServer(t, nil, map[string][]byte{"full.zip": full, "embeddable.zip": embeddable})

	archive, err := buildFromInstallManagerZip(srv.URL+"/ftp/full.zip", sha256Hex(full))
	if err != nil {
		t.Fatalf("buildFromInstallManagerZip() error: %v", err)
	}
	entries := tarEntries(t, archive)
	for rel, want := range map[string]string{
		"python.exe":       "exe",
		"__install__.json": "{}",
		"include/Python.h": "header",
		// Entry names are used as they are, not URI-unescaped.
		"Lib/script%20(dev).tmpl": "not escaped",
	} {
		if got, ok := entries["python/"+rel]; !ok || got != want {
			t.Errorf("archive entry python/%s = %q (present %v), want %q", rel, got, ok, want)
		}
	}

	if _, err := buildFromInstallManagerZip(srv.URL+"/ftp/full.zip", sha256Hex([]byte("other"))); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Errorf("buildFromInstallManagerZip(wrong hash) error = %v, want a SHA-256 mismatch", err)
	}
	if _, err := buildFromInstallManagerZip(srv.URL+"/ftp/embeddable.zip", sha256Hex(embeddable)); !errors.Is(err, errNotFullBuild) {
		t.Errorf("buildFromInstallManagerZip(embeddable) error = %v, want errNotFullBuild", err)
	}
}

func TestIsSHA256Hex(t *testing.T) {
	for s, want := range map[string]bool{
		strings.Repeat("0a", 32): true,
		strings.Repeat("0A", 32): true,
		strings.Repeat("0a", 31): false,
		strings.Repeat("zz", 32): false,
		"":                       false,
	} {
		if got := isSHA256Hex(s); got != want {
			t.Errorf("isSHA256Hex(%q) = %v, want %v", s, got, want)
		}
	}
}
