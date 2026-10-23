package main

// Tests against the real python.org and nuget.org. They download about
// 50 MB, and the MSI build needs msiextract, so they only run when asked:
//
//	MIRROR_NETWORK_TESTS=1 go test -v -run Network ./...
//
// On Debian or Ubuntu, install msiextract with: apt-get install msitools.
// Set MIRROR_NETWORK_OUT to a directory to keep the built archives, e.g. to
// unpack and run them on Windows.

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func requireNetworkTests(t *testing.T) {
	t.Helper()
	if os.Getenv("MIRROR_NETWORK_TESTS") != "1" {
		t.Skip("set MIRROR_NETWORK_TESTS=1 to run tests against python.org and nuget.org")
	}
}

// checkRealArchive confirms an archive holds a full install and keeps it
// in MIRROR_NETWORK_OUT if set.
func checkRealArchive(t *testing.T, name string, archive []byte, requireTk bool) {
	t.Helper()
	var files []string
	for entry := range tarEntries(t, archive) {
		if rel := entry[len("python/"):]; rel != "" && rel[len(rel)-1] != '/' {
			files = append(files, rel)
		}
	}
	if err := validateWindowsPython(files, requireTk); err != nil {
		t.Errorf("%s: %v", name, err)
	}
	t.Logf("%s: %d files, %.1f MB", name, len(files), float64(len(archive))/1e6)

	if dir := os.Getenv("MIRROR_NETWORK_OUT"); dir != "" {
		if err := os.WriteFile(filepath.Join(dir, name), archive, 0o644); err != nil {
			t.Errorf("saving %s: %v", name, err)
		}
	}
}

func TestNetwork_BuildFromMSIs(t *testing.T) {
	requireNetworkTests(t)
	if _, err := exec.LookPath("msiextract"); err != nil {
		t.Skip("msiextract not installed (apt-get install msitools)")
	}
	verifier, err := newPythonWindowsVerifier()
	if err != nil {
		t.Fatalf("newPythonWindowsVerifier: %v", err)
	}

	// The oldest and newest signed releases in range, and 3.9.11, which has
	// no NuGet package at all.
	for _, v := range []string{"3.5.0", "3.9.11", "3.12.2"} {
		archive, err := buildFromMSIs(pythonFTPBaseURL+v+"/amd64/", verifier, nil)
		if err != nil {
			t.Errorf("buildFromMSIs(%s): %v", v, err)
			continue
		}
		checkRealArchive(t, "python-"+v+"-windows-amd64.tar.gz", archive, true)
	}
}

func TestNetwork_BuildFromNuGet(t *testing.T) {
	requireNetworkTests(t)

	// The first pythonarm64 release, and one from the 3.11 line.
	for _, v := range []string{"3.9.7", "3.11.4"} {
		nupkgURL := nugetFlatContainer + "pythonarm64/" + v + "/pythonarm64." + v + ".nupkg"
		leafURL := nugetRegistrationURL + "pythonarm64/" + v + ".json"
		archive, err := buildFromNuGet(nupkgURL, leafURL)
		if err != nil {
			t.Errorf("buildFromNuGet(pythonarm64 %s): %v", v, err)
			continue
		}
		checkRealArchive(t, "python-"+v+"-windows-arm64.tar.gz", archive, false)
	}

	// x64 NuGet packages are not reliably full installs, which is why the
	// NuGet source skips them; the validator must say so.
	nupkgURL := nugetFlatContainer + "python/3.5.2/python.3.5.2.nupkg"
	leafURL := nugetRegistrationURL + "python/3.5.2.json"
	if _, err := buildFromNuGet(nupkgURL, leafURL); !errors.Is(err, errNotFullBuild) {
		t.Errorf("buildFromNuGet(python 3.5.2) = %v, want errNotFullBuild", err)
	}
}

func TestNetwork_FetchVersions(t *testing.T) {
	requireNetworkTests(t)

	msiJobs, err := (&PythonMSISource{}).FetchVersions()
	if err != nil {
		t.Fatalf("PythonMSISource.FetchVersions: %v", err)
	}
	msi := make(map[string]bool)
	for _, j := range msiJobs {
		msi[j.Version] = true
	}
	t.Logf("python.org MSIs: %d signed x64 releases", len(msiJobs))
	for _, v := range []string{"3.5.0", "3.9.11", "3.12.2", "3.13.1"} {
		if !msi[v] {
			t.Errorf("MSI source has no job for %s", v)
		}
	}
	for _, v := range []string{"3.5.2", "3.9.15", "3.14.0"} {
		if msi[v] {
			t.Errorf("MSI source has a job for %s, which has unsigned or no MSIs", v)
		}
	}

	nugetJobs, err := (&PythonNuGetSource{}).FetchVersions()
	if err != nil {
		t.Fatalf("PythonNuGetSource.FetchVersions: %v", err)
	}
	nuget := make(map[string]bool)
	for _, j := range nugetJobs {
		nuget[j.Version+"/"+j.Platform] = true
	}
	t.Logf("python.org NuGet packages: %d release jobs", len(nugetJobs))
	for _, key := range []string{"3.9.7/windows-arm64", "3.11.4/windows-arm64", "3.12.2/windows-arm64"} {
		if !nuget[key] {
			t.Errorf("NuGet source has no job for %s", key)
		}
	}
	for _, j := range nugetJobs {
		if j.Platform != "windows-arm64" {
			t.Errorf("NuGet source has a %s job (%s); it should only cover windows-arm64", j.Platform, j.Version)
		}
	}
}
