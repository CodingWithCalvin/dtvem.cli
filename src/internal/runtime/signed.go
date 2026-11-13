//go:build !shim

package runtime

import "fmt"

// NoSignedBuildError reports that there is no signed build of a version
// for a platform.
type NoSignedBuildError struct {
	// Runtime is the runtime's display name, e.g. "Python".
	Runtime string
	Version string
	// Build names the signed build, e.g. "python.org".
	Build    string
	Platform string
	// Nearest is the version nearest to Version on its major.minor line
	// that has a signed build, or "" if none does.
	Nearest string
	// NoneListed is set when there are no signed builds at all for the
	// platform.
	NoneListed bool
}

func (e *NoSignedBuildError) Error() string {
	if e.NoneListed {
		return fmt.Sprintf("the %s manifest lists no signed (%s) builds for %s", e.Runtime, e.Build, e.Platform)
	}
	msg := fmt.Sprintf("%s %s has no signed (%s) build for %s", e.Runtime, e.Version, e.Build, e.Platform)
	if e.Nearest != "" {
		msg += fmt.Sprintf("; the nearest version with one is %s", e.Nearest)
	}
	return msg
}

// BuildInfo says which build of a runtime an installed version came from.
type BuildInfo struct {
	// Name is the build, such as "python.org" or "python-build-standalone".
	// Empty when dtvem cannot tell.
	Name string

	// Signed reports whether the build's executables carry their
	// publisher's Authenticode signature.
	Signed bool
}

// SignedBuildProvider is implemented by providers that can install a
// version's publisher-signed build in place of the build they install by
// default: Python on Windows, where python.org's build is Authenticode-
// signed by the Python Software Foundation and python-build-standalone's
// is not.
type SignedBuildProvider interface {
	// SignedBuildsSupported reports whether signed builds are offered on
	// this platform.
	SignedBuildsSupported() bool

	// SignedVersions returns the versions that have a signed build for
	// this platform, newest first.
	SignedVersions() ([]string, error)

	// InstallSigned installs the signed build of version. It fails with a
	// *NoSignedBuildError when version has no signed build.
	InstallSigned(version string) error

	// InstalledBuild reports which build an installed version came from.
	InstalledBuild(version string) BuildInfo
}
