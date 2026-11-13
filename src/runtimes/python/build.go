//go:build !shim

// Which build of Python an install comes from. On Windows dtvem installs
// python-build-standalone's build by default, whose executables are not
// Authenticode-signed; `dtvem install python <version> --signed` installs
// python.org's own build instead, signed by the Python Software Foundation,
// for the versions python.org shipped Windows binaries for.

package python

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"sync"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/manifest"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/version"
)

// buildMarkerFile, in an install's directory, records which build was
// installed there. Installs made before dtvem wrote it have none.
const buildMarkerFile = ".dtvem-build.json"

// buildMarker is the content of buildMarkerFile.
type buildMarker struct {
	Build  string `json:"build"`
	URL    string `json:"url"`
	SHA256 string `json:"sha256,omitempty"`
}

// SignedBuildsSupported reports whether signed builds are offered here:
// python.org's builds dtvem mirrors are Windows builds.
func (p *Provider) SignedBuildsSupported() bool {
	return goruntime.GOOS == constants.OSWindows
}

// SignedVersions returns the versions with a python.org build for this
// platform, newest first.
func (p *Provider) SignedVersions() ([]string, error) {
	if !p.SignedBuildsSupported() {
		return nil, nil
	}
	m, err := manifest.DefaultSource().GetManifest("python")
	if err != nil {
		return nil, fmt.Errorf("failed to load manifest: %w", err)
	}
	return signedVersions(m, manifest.CurrentPlatform()), nil
}

// InstalledBuild reports which build an installed version came from: what
// the install recorded, or for an install from before dtvem recorded it,
// the build its version's default download is in the manifest built into
// dtvem (the default downloads of a version do not change).
func (p *Provider) InstalledBuild(ver string) runtime.BuildInfo {
	name := readBuildMarker(config.RuntimeVersionPath("python", ver))
	if name == "" {
		if m, err := embeddedPythonManifest(); err == nil {
			name = defaultBuild(m, ver, manifest.CurrentPlatform())
		}
	}
	return runtime.BuildInfo{Name: name, Signed: name == manifest.BuildPythonOrg}
}

// alreadyInstalledError explains why a version that is installed is not
// installed again, saying how to replace it with the build asked for.
func (p *Provider) alreadyInstalledError(ver, build string) error {
	if build == "" {
		return fmt.Errorf("Python %s is already installed", ver)
	}
	installed := p.InstalledBuild(ver).Name
	if installed == build {
		return fmt.Errorf("Python %s is already installed (%s build)", ver, build)
	}
	from := "dtvem cannot tell from which build"
	if installed != "" {
		from = fmt.Sprintf("from %s", installed)
	}
	return fmt.Errorf("Python %s is already installed, %s; to replace it with the %s build, run `dtvem uninstall python %s`, then install it again", ver, from, build, ver)
}

// selectDownload returns the download of a version for a platform: the
// named build, or the default download when build is empty.
func selectDownload(m *manifest.Manifest, ver, platform, build string) (*manifest.Download, error) {
	dl := m.GetDownload(ver, platform)
	if build == "" {
		if dl == nil {
			return nil, fmt.Errorf("Python %s is not available for %s", ver, platform)
		}
		return dl, nil
	}

	if b := dl.ForBuild(build); b != nil {
		return b, nil
	}
	available := m.ListBuildVersions(platform, build)
	return nil, &runtime.NoSignedBuildError{
		Runtime:    "Python",
		Version:    ver,
		Build:      build,
		Platform:   platform,
		Nearest:    version.Nearest(ver, available),
		NoneListed: len(available) == 0,
	}
}

// signedVersions returns the versions with a python.org build for a
// platform, newest first.
func signedVersions(m *manifest.Manifest, platform string) []string {
	versions := m.ListBuildVersions(platform, manifest.BuildPythonOrg)
	sort.Slice(versions, func(i, j int) bool {
		return version.Compare(versions[i], versions[j]) > 0
	})
	return versions
}

// defaultBuild returns the build of a version's default download for a
// platform, or "" when the manifest does not say.
func defaultBuild(m *manifest.Manifest, ver, platform string) string {
	if dl := m.GetDownload(ver, platform); dl != nil {
		return dl.Build
	}
	return ""
}

// embeddedPythonManifest is the Python manifest built into dtvem, parsed
// once.
var embeddedPythonManifest = sync.OnceValues(func() (*manifest.Manifest, error) {
	return manifest.NewEmbeddedSource().GetManifest("python")
})

// writeBuildMarker records in installPath which build dl is. It writes
// nothing when the manifest did not say.
func writeBuildMarker(installPath string, dl *manifest.Download) error {
	if dl.Build == "" {
		return nil
	}
	data, err := json.MarshalIndent(buildMarker{Build: dl.Build, URL: dl.URL, SHA256: dl.SHA256}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(installPath, buildMarkerFile), data, 0o644)
}

// readBuildMarker returns the build recorded in installPath, or "" when
// there is no readable record.
func readBuildMarker(installPath string) string {
	data, err := os.ReadFile(filepath.Join(installPath, buildMarkerFile))
	if err != nil {
		return ""
	}
	var m buildMarker
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m.Build
}
