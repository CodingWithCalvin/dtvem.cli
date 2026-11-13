package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// mockSignedProvider is a mockProvider that also offers signed builds.
type mockSignedProvider struct {
	mockProvider
	supported      bool
	signedVersions []string
	signedErr      error
	installedBuild map[string]runtime.BuildInfo
	signedCalls    []string
}

func (m *mockSignedProvider) SignedBuildsSupported() bool { return m.supported }
func (m *mockSignedProvider) SignedVersions() ([]string, error) {
	return m.signedVersions, m.signedErr
}
func (m *mockSignedProvider) InstallSigned(version string) error {
	m.signedCalls = append(m.signedCalls, version)
	return nil
}
func (m *mockSignedProvider) InstalledBuild(version string) runtime.BuildInfo {
	return m.installedBuild[version]
}

func newMockPython(supported bool) *mockSignedProvider {
	return &mockSignedProvider{
		mockProvider: mockProvider{
			name:        "python",
			displayName: "Python",
			availableVersions: []runtime.AvailableVersion{
				{Version: runtime.NewVersion("3.13.9")},
				{Version: runtime.NewVersion("3.12.12")},
				{Version: runtime.NewVersion("3.12.11")},
				{Version: runtime.NewVersion("3.12.10")},
				{Version: runtime.NewVersion("3.11.9")},
			},
		},
		supported:      supported,
		signedVersions: []string{"3.13.9", "3.12.10", "3.12.9"},
	}
}

func TestSignedBuildProvider(t *testing.T) {
	node := &mockProvider{name: "node", displayName: "Node.js"}
	if _, err := signedBuildProvider(node); err == nil || !strings.Contains(err.Error(), "only supported for Python, not Node.js") {
		t.Errorf("signedBuildProvider(node) error = %v, want only-Python error", err)
	}

	if _, err := signedBuildProvider(newMockPython(false)); err == nil || !strings.Contains(err.Error(), "only supported on Windows") {
		t.Errorf("signedBuildProvider(python, unsupported) error = %v, want only-Windows error", err)
	}

	python := newMockPython(true)
	got, err := signedBuildProvider(python)
	if err != nil || got != python {
		t.Errorf("signedBuildProvider(python) = %v, %v; want the provider", got, err)
	}
}

func TestResolveSignedVersion(t *testing.T) {
	python := newMockPython(true)

	tests := []struct {
		input   string
		want    string
		wantErr string
	}{
		// A full version is installed as asked; the install reports a
		// missing signed build.
		{input: "3.12.12", want: "3.12.12"},
		{input: "v3.12.10", want: "3.12.10"},
		// A partial version picks the newest signed build, not the newest
		// release (3.12.12).
		{input: "3.12", want: "3.12.10"},
		{input: "3", want: "3.13.9"},
		{input: "3.11", wantErr: `no Python version matching "3.11" has a signed build; 'dtvem list-all python' shows which versions do`},
	}
	for _, tt := range tests {
		got, err := resolveSignedVersion(python, python, tt.input)
		if tt.wantErr != "" {
			if err == nil || err.Error() != tt.wantErr {
				t.Errorf("resolveSignedVersion(%q) error = %v, want %q", tt.input, err, tt.wantErr)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("resolveSignedVersion(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
		}
	}

	python.signedErr = errors.New("offline")
	if _, err := resolveSignedVersion(python, python, "3.12"); err == nil || !strings.Contains(err.Error(), "offline") {
		t.Errorf("resolveSignedVersion() with failing SignedVersions error = %v, want it passed on", err)
	}
}

func TestNewerUnsigned(t *testing.T) {
	python := newMockPython(true)
	python.availableVersions = append(python.availableVersions, runtime.AvailableVersion{Version: runtime.NewVersion("3.12.9")})

	tests := []struct {
		installed string
		want      string
	}{
		// 3.12.10 has a signed build, so only 3.12.11 and 3.12.12 count.
		{"3.12.9", "3.12.12,3.12.11"},
		{"3.12.10", "3.12.12,3.12.11"},
		{"3.13.9", ""},
		// Other lines never count.
		{"3.11.9", ""},
	}
	for _, tt := range tests {
		got := newerUnsigned(tt.installed, python.availableVersions, python.signedVersions)
		if strings.Join(got, ",") != tt.want {
			t.Errorf("newerUnsigned(%s) = %v, want %s", tt.installed, got, tt.want)
		}
	}
}

func TestNewerUnsignedWarning(t *testing.T) {
	if got, want := newerUnsignedWarning("Python", "3.12.10", []string{"3.12.11", "3.12.12"}),
		"Python 3.12.11 through 3.12.12 are newer but have no signed build; they may carry security fixes 3.12.10 lacks"; got != want {
		t.Errorf("newerUnsignedWarning(two) = %q, want %q", got, want)
	}
	if got, want := newerUnsignedWarning("Python", "3.12.10", []string{"3.12.11"}),
		"Python 3.12.11 is newer but has no signed build; it may carry security fixes 3.12.10 lacks"; got != want {
		t.Errorf("newerUnsignedWarning(one) = %q, want %q", got, want)
	}
}

func TestDescribeBuild(t *testing.T) {
	for _, tt := range []struct {
		info runtime.BuildInfo
		want string
	}{
		{runtime.BuildInfo{Name: "python.org", Signed: true}, "python.org, signed"},
		{runtime.BuildInfo{Name: "python-build-standalone"}, "python-build-standalone, not signed"},
		{runtime.BuildInfo{}, "unknown"},
	} {
		if got := describeBuild(tt.info); got != tt.want {
			t.Errorf("describeBuild(%+v) = %q, want %q", tt.info, got, tt.want)
		}
	}
}

func TestCheckSignedTasks(t *testing.T) {
	node := &mockProvider{name: "node", displayName: "Node.js"}

	// Runtimes without signed builds install as usual.
	ok := []installTask{{provider: node}, {provider: newMockPython(true), signed: newMockPython(true)}}
	if err := checkSignedTasks(ok); err != nil {
		t.Errorf("checkSignedTasks(supported) error = %v, want nil", err)
	}

	unsupported := newMockPython(false)
	bad := []installTask{{provider: node}, {provider: unsupported, signed: unsupported}}
	if err := checkSignedTasks(bad); err == nil {
		t.Error("checkSignedTasks(unsupported) error = nil, want an error rather than unsigned installs")
	}
}

func TestShowNoSignedBuildHelp_IgnoresOtherErrors(t *testing.T) {
	// Only a *runtime.NoSignedBuildError gets help; this must not panic.
	showNoSignedBuildHelp(newMockPython(true), errors.New("download failed"))
	showNoSignedBuildHelp(newMockPython(true), &runtime.NoSignedBuildError{Runtime: "Python", Version: "3.12.12", Build: "python.org", Platform: "windows-amd64", Nearest: "3.12.10"})
}

func TestNoSignedBuildError(t *testing.T) {
	err := &runtime.NoSignedBuildError{Runtime: "Python", Version: "3.12.12", Build: "python.org", Platform: "windows-amd64", Nearest: "3.12.10"}
	if want := "Python 3.12.12 has no signed (python.org) build for windows-amd64; the nearest version with one is 3.12.10"; err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
	none := &runtime.NoSignedBuildError{Runtime: "Python", Build: "python.org", Platform: "windows-amd64", NoneListed: true}
	if want := "the Python manifest lists no signed (python.org) builds for windows-amd64"; none.Error() != want {
		t.Errorf("Error() = %q, want %q", none.Error(), want)
	}
}
