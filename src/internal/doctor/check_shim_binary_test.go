package doctor

import (
	"errors"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// shimFilename is the on-disk name of the shim helper for the current
// platform, used by tests to construct realistic install layouts.
func shimFilename() string {
	if goruntime.GOOS == constants.OSWindows {
		return "dtvem-shim" + constants.ExtExe
	}
	return "dtvem-shim"
}

func TestShimBinaryCheck_Present(t *testing.T) {
	installDir := t.TempDir()
	dtvemExe := filepath.Join(installDir, "dtvem")
	shimExe := filepath.Join(installDir, shimFilename())

	// The check looks for dtvem-shim next to whatever os.Executable
	// returns, so we plant a real file at the expected location.
	if err := os.WriteFile(shimExe, []byte("stub"), 0644); err != nil {
		t.Fatalf("write shim: %v", err)
	}

	c := newShimBinaryCheck()
	c.executable = func() (string, error) { return dtvemExe, nil }

	got := c.Run()
	if !got.OK {
		t.Errorf("expected OK when shim binary exists, got %#v", got)
	}
}

func TestShimBinaryCheck_Missing(t *testing.T) {
	installDir := t.TempDir()
	dtvemExe := filepath.Join(installDir, "dtvem")
	// Note: we deliberately do not write dtvem-shim into installDir.

	c := newShimBinaryCheck()
	c.executable = func() (string, error) { return dtvemExe, nil }

	got := c.Run()
	if got.OK {
		t.Fatalf("expected non-OK when shim binary missing, got OK")
	}
	if got.Severity != SeverityError {
		t.Errorf("severity: got %s, want error", got.Severity)
	}
	if got.Fixable() {
		t.Errorf("shim-binary check should be manual, but Finding.Fix is set")
	}

	// The expected location must be in details so the user knows where
	// dtvem looked and didn't find it.
	expectedAt := filepath.Join(installDir, shimFilename())
	foundDetail := false
	for _, d := range got.Details {
		if d.Value == expectedAt {
			foundDetail = true
			break
		}
	}
	if !foundDetail {
		t.Errorf("expected %q in details, got %#v", expectedAt, got.Details)
	}
}

func TestShimBinaryCheck_DirectoryAtExpectedPathIsTreatedAsMissing(t *testing.T) {
	// Edge case: if a directory happens to share the shim's path (e.g.
	// because the user manually created one), the file isn't actually
	// usable, so we should report it as missing rather than present.
	installDir := t.TempDir()
	dtvemExe := filepath.Join(installDir, "dtvem")
	shimPath := filepath.Join(installDir, shimFilename())
	if err := os.Mkdir(shimPath, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	c := newShimBinaryCheck()
	c.executable = func() (string, error) { return dtvemExe, nil }

	got := c.Run()
	if got.OK {
		t.Errorf("expected non-OK when a directory occupies the shim's path, got OK")
	}
}

func TestShimBinaryCheck_ExecutableLookupFailure(t *testing.T) {
	c := newShimBinaryCheck()
	c.executable = func() (string, error) { return "", errors.New("boom") }

	got := c.Run()
	if got.OK {
		t.Fatalf("expected non-OK finding when executable lookup fails")
	}
	if got.Severity != SeverityWarning {
		// Warning rather than error: we couldn't run the check, but
		// that doesn't necessarily mean the shim binary is missing.
		t.Errorf("severity: got %s, want warning", got.Severity)
	}
}

func TestShimBinaryCheck_ResolutionPlatformSpecific(t *testing.T) {
	got := shimBinaryResolution()
	if !strings.Contains(got, "dtvem.io") {
		t.Errorf("resolution should reference the installer URL, got %q", got)
	}
	if goruntime.GOOS == constants.OSWindows {
		if !strings.Contains(got, "install.ps1") {
			t.Errorf("Windows resolution should reference install.ps1, got %q", got)
		}
	} else {
		if !strings.Contains(got, "install.sh") {
			t.Errorf("Unix resolution should reference install.sh, got %q", got)
		}
	}
}

func TestShimBinaryCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "shim-binary-present" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("shim-binary-present check is not in the default registry")
	}
}
