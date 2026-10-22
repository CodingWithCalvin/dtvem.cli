package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// pythonInstalls creates a python versions directory with one install per
// entry, adding a pythonXY._pth file to the versions listed in embeddable.
func pythonInstalls(t *testing.T, versions []string, embeddable map[string]bool) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "versions", "python")
	for _, v := range versions {
		dir := filepath.Join(root, v)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "python.exe"), nil, 0o644); err != nil {
			t.Fatalf("writing python.exe: %v", err)
		}
		if embeddable[v] {
			parts := strings.Split(v, ".")
			name := "python" + parts[0] + parts[1] + "._pth"
			if err := os.WriteFile(filepath.Join(dir, name), []byte("python39.zip\n.\nimport site\n"), 0o644); err != nil {
				t.Fatalf("writing %s: %v", name, err)
			}
		}
	}
	return root
}

func TestPythonPthCheck_NoPythonInstalled(t *testing.T) {
	c := pythonPthCheck{pythonVersionsDir: func() string { return filepath.Join(t.TempDir(), "missing") }}

	if f := c.Run(); !f.OK {
		t.Errorf("Run() = %+v, want OK", f)
	}
}

func TestPythonPthCheck_FullBuilds(t *testing.T) {
	root := pythonInstalls(t, []string{"3.12.12", "3.13.1"}, nil)
	c := pythonPthCheck{pythonVersionsDir: func() string { return root }}

	if f := c.Run(); !f.OK {
		t.Errorf("Run() = %+v, want OK", f)
	}
}

func TestPythonPthCheck_FlagsAndFixesEmbeddableInstalls(t *testing.T) {
	root := pythonInstalls(t, []string{"3.12.12", "3.9.9", "3.8.9"}, map[string]bool{"3.9.9": true, "3.8.9": true})
	c := pythonPthCheck{pythonVersionsDir: func() string { return root }}

	f := c.Run()
	if f.OK || f.Severity != SeverityWarning {
		t.Fatalf("Run() OK=%v severity=%v, want a warning", f.OK, f.Severity)
	}
	if !f.Fixable() {
		t.Fatal("finding is not fixable, want the rename as a fix")
	}
	want := []Detail{
		{Key: "Python 3.8.9", Value: "python38._pth"},
		{Key: "Python 3.9.9", Value: "python39._pth"},
	}
	if len(f.Details) != len(want) {
		t.Fatalf("Details = %v, want %v", f.Details, want)
	}
	for i := range want {
		if f.Details[i] != want[i] {
			t.Errorf("Details[%d] = %v, want %v", i, f.Details[i], want[i])
		}
	}
	for _, s := range []string{"._pth.disabled", "dtvem install python", "3.8.9, 3.9.9"} {
		if !strings.Contains(f.Resolution, s) {
			t.Errorf("Resolution %q does not mention %q", f.Resolution, s)
		}
	}

	if err := f.Fix(); err != nil {
		t.Fatalf("Fix() error: %v", err)
	}
	for _, v := range []string{"3.8.9", "3.9.9"} {
		parts := strings.Split(v, ".")
		pthFile := filepath.Join(root, v, "python"+parts[0]+parts[1]+"._pth")
		if _, err := os.Stat(pthFile); !os.IsNotExist(err) {
			t.Errorf("%s still exists after Fix()", pthFile)
		}
		if _, err := os.Stat(pthFile + ".disabled"); err != nil {
			t.Errorf("%s.disabled missing after Fix(): %v", pthFile, err)
		}
	}

	if after := c.Run(); !after.OK {
		t.Errorf("Run() after Fix() = %+v, want OK", after)
	}
}

func TestPythonPthCheck_Registered(t *testing.T) {
	for _, c := range All() {
		if c.Name() == "python-isolated-mode" {
			return
		}
	}
	t.Error("python-isolated-mode check is not registered")
}
