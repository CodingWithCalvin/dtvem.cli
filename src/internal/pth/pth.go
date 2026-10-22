// Package pth handles the ._pth file in python.org's embeddable Windows
// Python package.
//
// While a pythonXY._pth file sits next to python.exe, Python builds sys.path
// from that file alone and runs in isolated mode: a script's own directory is
// not on sys.path, PYTHONPATH is ignored, and site-packages is off. That suits
// a runtime embedded in an application, not one people run directly. Full
// builds have no ._pth file. Renaming it to pythonXY._pth.disabled makes an
// embeddable install behave like a regular one, and the rename can be undone.
package pth

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// DisabledSuffix is appended to a ._pth file's name to turn it off. Python
// only looks for the exact ._pth name.
const DisabledSuffix = ".disabled"

// Find returns the ._pth files directly inside a Python install directory.
func Find(installDir string) ([]string, error) {
	return filepath.Glob(filepath.Join(installDir, "*._pth"))
}

// Disable renames every ._pth file in a Python install directory to
// <name>.disabled and returns the files it renamed. A leftover .disabled
// copy from an earlier rename is replaced.
func Disable(installDir string) ([]string, error) {
	files, err := Find(installDir)
	if err != nil {
		return nil, err
	}

	var disabled []string
	for _, f := range files {
		target := f + DisabledSuffix
		if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return disabled, fmt.Errorf("removing old %s: %w", filepath.Base(target), err)
		}
		if err := os.Rename(f, target); err != nil {
			return disabled, fmt.Errorf("disabling %s: %w", filepath.Base(f), err)
		}
		disabled = append(disabled, f)
	}
	return disabled, nil
}
