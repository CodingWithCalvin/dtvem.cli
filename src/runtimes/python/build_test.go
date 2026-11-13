package python

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/manifest"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

const testPlatform = "windows-amd64"

// testBuildsManifest has a python-build-standalone default with a
// python.org alternate (3.12.10, 3.13.9), a python.org default (3.9.13) and
// versions with no python.org build (3.12.12, 3.11.0).
func testBuildsManifest() *manifest.Manifest {
	pbs := func(v string) *manifest.Download {
		return &manifest.Download{URL: "https://builds.dtvem.io/python/" + v + "/windows-amd64.tar.gz", SHA256: "pbs-" + v, Build: manifest.BuildPythonStandalone}
	}
	org := func(v string) *manifest.Download {
		return &manifest.Download{URL: "https://builds.dtvem.io/python/" + v + "/python.org/windows-amd64.tar.gz", SHA256: "org-" + v, Build: manifest.BuildPythonOrg}
	}
	withOrg := func(v string) *manifest.Download {
		d := pbs(v)
		d.Alternates = []*manifest.Download{org(v)}
		return d
	}
	return &manifest.Manifest{
		Version: 1,
		Versions: map[string]map[string]*manifest.Download{
			"3.12.10": {testPlatform: withOrg("3.12.10"), "linux-amd64": pbs("3.12.10")},
			"3.12.12": {testPlatform: pbs("3.12.12")},
			"3.13.9":  {testPlatform: withOrg("3.13.9")},
			"3.9.13": {testPlatform: {
				URL: "https://builds.dtvem.io/python/3.9.13/windows-amd64.tar.gz", SHA256: "org-default", Build: manifest.BuildPythonOrg,
			}},
			"3.11.0": {testPlatform: {URL: "https://builds.dtvem.io/python/3.11.0/windows-amd64.tar.gz"}},
		},
	}
}

func TestSelectDownload(t *testing.T) {
	m := testBuildsManifest()

	tests := []struct {
		name        string
		version     string
		platform    string
		build       string
		wantURL     string
		wantNearest string // for a *NoSignedBuildError
		wantErr     string
	}{
		{name: "default", version: "3.12.10", platform: testPlatform, wantURL: "https://builds.dtvem.io/python/3.12.10/windows-amd64.tar.gz"},
		{name: "default, unlabeled", version: "3.11.0", platform: testPlatform, wantURL: "https://builds.dtvem.io/python/3.11.0/windows-amd64.tar.gz"},
		{name: "default, missing", version: "3.99.0", platform: testPlatform, wantErr: "Python 3.99.0 is not available for windows-amd64"},
		{name: "signed alternate", version: "3.12.10", platform: testPlatform, build: manifest.BuildPythonOrg, wantURL: "https://builds.dtvem.io/python/3.12.10/python.org/windows-amd64.tar.gz"},
		{name: "signed default", version: "3.9.13", platform: testPlatform, build: manifest.BuildPythonOrg, wantURL: "https://builds.dtvem.io/python/3.9.13/windows-amd64.tar.gz"},
		{name: "no signed build: security-only release", version: "3.12.12", platform: testPlatform, build: manifest.BuildPythonOrg, wantNearest: "3.12.10",
			wantErr: "Python 3.12.12 has no signed (python.org) build for windows-amd64; the nearest version with one is 3.12.10"},
		{name: "no signed build on the line", version: "3.11.0", platform: testPlatform, build: manifest.BuildPythonOrg,
			wantErr: "Python 3.11.0 has no signed (python.org) build for windows-amd64"},
		{name: "none listed for platform", version: "3.12.10", platform: "linux-amd64", build: manifest.BuildPythonOrg,
			wantErr: "the Python manifest lists no signed (python.org) builds for linux-amd64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dl, err := selectDownload(m, tt.version, tt.platform, tt.build)
			if tt.wantErr != "" {
				if err == nil || err.Error() != tt.wantErr {
					t.Fatalf("selectDownload() error = %v, want %q", err, tt.wantErr)
				}
				var noSigned *runtime.NoSignedBuildError
				if tt.build != "" && (!errors.As(err, &noSigned) || noSigned.Nearest != tt.wantNearest) {
					t.Errorf("selectDownload() error = %#v, want a *runtime.NoSignedBuildError with Nearest %q", err, tt.wantNearest)
				}
				return
			}
			if err != nil {
				t.Fatalf("selectDownload() error = %v", err)
			}
			if dl.URL != tt.wantURL {
				t.Errorf("selectDownload() URL = %q, want %q", dl.URL, tt.wantURL)
			}
		})
	}
}

func TestSignedVersions(t *testing.T) {
	got := signedVersions(testBuildsManifest(), testPlatform)
	if want := "3.13.9,3.12.10,3.9.13"; strings.Join(got, ",") != want {
		t.Errorf("signedVersions() = %v, want %s (newest first)", got, want)
	}
	if got := signedVersions(testBuildsManifest(), "linux-amd64"); len(got) != 0 {
		t.Errorf("signedVersions(linux-amd64) = %v, want none", got)
	}
}

func TestDefaultBuild(t *testing.T) {
	m := testBuildsManifest()
	for _, tt := range []struct{ version, platform, want string }{
		{"3.12.10", testPlatform, manifest.BuildPythonStandalone},
		{"3.9.13", testPlatform, manifest.BuildPythonOrg},
		{"3.11.0", testPlatform, ""},
		{"3.99.0", testPlatform, ""},
	} {
		if got := defaultBuild(m, tt.version, tt.platform); got != tt.want {
			t.Errorf("defaultBuild(%s, %s) = %q, want %q", tt.version, tt.platform, got, tt.want)
		}
	}
}

func TestBuildMarker(t *testing.T) {
	dir := t.TempDir()

	if got := readBuildMarker(dir); got != "" {
		t.Errorf("readBuildMarker(no marker) = %q, want \"\"", got)
	}

	// The manifest did not say which build: nothing is recorded.
	if err := writeBuildMarker(dir, &manifest.Download{URL: "https://example.invalid/x.tar.gz"}); err != nil {
		t.Fatalf("writeBuildMarker(unlabeled) error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, buildMarkerFile)); !os.IsNotExist(err) {
		t.Errorf("writeBuildMarker(unlabeled) wrote %s (stat error %v)", buildMarkerFile, err)
	}

	dl := testBuildsManifest().GetDownload("3.12.10", testPlatform).ForBuild(manifest.BuildPythonOrg)
	if err := writeBuildMarker(dir, dl); err != nil {
		t.Fatalf("writeBuildMarker() error: %v", err)
	}
	if got := readBuildMarker(dir); got != manifest.BuildPythonOrg {
		t.Errorf("readBuildMarker() = %q, want %q", got, manifest.BuildPythonOrg)
	}

	if err := os.WriteFile(filepath.Join(dir, buildMarkerFile), []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := readBuildMarker(dir); got != "" {
		t.Errorf("readBuildMarker(corrupt) = %q, want \"\"", got)
	}
}

// setTestRoot points dtvem at an empty root for the test.
func setTestRoot(t *testing.T) {
	t.Helper()
	t.Setenv("DTVEM_ROOT", t.TempDir())
	config.ResetPathsCache()
	t.Cleanup(config.ResetPathsCache)
}

// installFake makes an install directory for version holding a marker for
// build, or no marker when build is empty.
func installFake(t *testing.T, version, build string) {
	t.Helper()
	dir := config.RuntimeVersionPath("python", version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if build != "" {
		if err := writeBuildMarker(dir, &manifest.Download{URL: "https://example.invalid/x.tar.gz", Build: build}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestInstalledBuild(t *testing.T) {
	setTestRoot(t)
	p := NewProvider()

	installFake(t, "3.12.10", manifest.BuildPythonOrg)
	installFake(t, "3.12.12", manifest.BuildPythonStandalone)

	if got, want := p.InstalledBuild("3.12.10"), (runtime.BuildInfo{Name: manifest.BuildPythonOrg, Signed: true}); got != want {
		t.Errorf("InstalledBuild(3.12.10) = %+v, want %+v", got, want)
	}
	if got, want := p.InstalledBuild("3.12.12"), (runtime.BuildInfo{Name: manifest.BuildPythonStandalone}); got != want {
		t.Errorf("InstalledBuild(3.12.12) = %+v, want %+v", got, want)
	}
	// Without a marker the answer comes from the built-in manifest, which
	// knows nothing of a version that does not exist.
	installFake(t, "0.0.1", "")
	if got := p.InstalledBuild("0.0.1"); got != (runtime.BuildInfo{}) {
		t.Errorf("InstalledBuild(0.0.1) = %+v, want unknown", got)
	}
}

func TestAlreadyInstalledError(t *testing.T) {
	setTestRoot(t)
	p := NewProvider()
	installFake(t, "3.12.10", manifest.BuildPythonOrg)
	installFake(t, "3.12.12", manifest.BuildPythonStandalone)
	installFake(t, "0.0.1", "")

	tests := []struct {
		version, build, want string
	}{
		{"3.12.12", "", "Python 3.12.12 is already installed"},
		{"3.12.10", manifest.BuildPythonOrg, "Python 3.12.10 is already installed (python.org build)"},
		{"3.12.12", manifest.BuildPythonOrg, "Python 3.12.12 is already installed, from python-build-standalone; to replace it with the python.org build, run `dtvem uninstall python 3.12.12`, then install it again"},
		{"0.0.1", manifest.BuildPythonOrg, "Python 0.0.1 is already installed, dtvem cannot tell from which build; to replace it with the python.org build, run `dtvem uninstall python 0.0.1`, then install it again"},
	}
	for _, tt := range tests {
		if got := p.alreadyInstalledError(tt.version, tt.build).Error(); got != tt.want {
			t.Errorf("alreadyInstalledError(%s, %q) = %q, want %q", tt.version, tt.build, got, tt.want)
		}
	}
}

func TestSignedProviderInterface(t *testing.T) {
	var _ runtime.SignedBuildProvider = NewProvider()
}
