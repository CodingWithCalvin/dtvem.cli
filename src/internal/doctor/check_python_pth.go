package doctor

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/pth"
)

// pythonPthCheck flags installed Python versions that still have a ._pth
// file, the mark of python.org's embeddable Windows package. While it is
// there Python runs in isolated mode: a script can't import modules sitting
// next to it, and PYTHONPATH is ignored.
//
// Older dtvem releases installed embeddable builds for some Windows
// versions. The mirror now serves full builds and the installer turns the
// file off, but versions installed before that keep it until fixed.
//
// The fix renames the file to ._pth.disabled, which restores normal
// imports in place. Only a reinstall brings the rest of a full build (venv,
// C headers, tkinter), so the resolution recommends that too.
type pythonPthCheck struct {
	pythonVersionsDir func() string
}

func newPythonPthCheck() *pythonPthCheck {
	return &pythonPthCheck{
		pythonVersionsDir: func() string { return filepath.Join(config.DefaultPaths().Versions, "python") },
	}
}

func (pythonPthCheck) Name() string { return "python-isolated-mode" }

func (c pythonPthCheck) Run() Finding {
	root := c.pythonVersionsDir()
	entries, err := os.ReadDir(root)
	if errors.Is(err, fs.ErrNotExist) {
		return Finding{OK: true, Title: "No Python versions installed"}
	}
	if err != nil {
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not list installed Python versions",
			Details:    []Detail{{Key: "Error", Value: err.Error()}},
			Resolution: "Check that " + root + " is readable.",
		}
	}

	type isolatedInstall struct {
		version string
		dir     string
		files   []string
	}
	var isolated []isolatedInstall
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		files, err := pth.Find(dir)
		if err != nil || len(files) == 0 {
			continue
		}
		isolated = append(isolated, isolatedInstall{version: e.Name(), dir: dir, files: files})
	}

	if len(isolated) == 0 {
		return Finding{OK: true, Title: "No installed Python version runs in isolated mode"}
	}

	sort.Slice(isolated, func(i, j int) bool { return isolated[i].version < isolated[j].version })
	details := make([]Detail, 0, len(isolated))
	var versions []string
	for _, inst := range isolated {
		names := make([]string, len(inst.files))
		for i, f := range inst.files {
			names[i] = filepath.Base(f)
		}
		details = append(details, Detail{Key: "Python " + inst.version, Value: strings.Join(names, ", ")})
		versions = append(versions, inst.version)
	}

	return Finding{
		Severity: SeverityWarning,
		Title:    "Python versions run in isolated mode (embeddable builds)",
		Details:  details,
		Resolution: "Rename their ._pth files to ._pth.disabled so scripts can import modules next to them and PYTHONPATH works. " +
			"For a full build with venv, C headers and tkinter, reinstall: dtvem uninstall python <version>, then dtvem install python <version> (" +
			strings.Join(versions, ", ") + ").",
		Fix: func() error {
			for _, inst := range isolated {
				if _, err := pth.Disable(inst.dir); err != nil {
					return fmt.Errorf("Python %s: %w", inst.version, err)
				}
			}
			return nil
		},
	}
}

func init() {
	Register(newPythonPthCheck())
}
