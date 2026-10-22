package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/version"
)

// venvDir is the directory uv and most tools create a project's virtual
// environment in.
const venvDir = ".venv"

// venvConfigFile is the file in a virtual environment that records the
// Python it was built from.
const venvConfigFile = "pyvenv.cfg"

// resolveVenv returns the Python version the virtual environment in dir's
// .venv was built from. The bool is false when dir has no project virtual
// environment: no .venv directory, a .venv without a pyvenv.cfg, or a .venv
// in the home directory, which is a user-wide environment rather than a
// project's.
func resolveVenv(dir, runtimeName string) (Resolution, bool, error) {
	venv := filepath.Join(dir, venvDir)
	info, err := os.Stat(venv)
	if errors.Is(err, fs.ErrNotExist) {
		return Resolution{}, false, nil
	}
	if err != nil {
		return Resolution{}, false, fmt.Errorf("could not read %s: %w", venv, err)
	}
	if !info.IsDir() || isHomeDir(dir) {
		return Resolution{}, false, nil
	}

	path := filepath.Join(venv, venvConfigFile)
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Resolution{}, false, nil
	}
	if err != nil {
		return Resolution{}, false, fmt.Errorf("could not read %s: %w", path, err)
	}

	venvVersion, err := parseVenvVersion(path, string(data))
	if err != nil {
		return Resolution{}, false, err
	}
	return Resolution{Version: matchVenvVersion(runtimeName, venvVersion), Source: path}, true, nil
}

// parseVenvVersion returns the version of the Python a virtual environment
// was built from, given the contents of its pyvenv.cfg at path. It reads the
// version key, which python -m venv writes, falling back to version_info,
// which uv writes, and keeps the leading major.minor[.patch]: a value can
// carry more, as in "3.12.1.final.0" or "3.13.0rc1".
func parseVenvVersion(path, content string) (string, error) {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			values[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}

	raw := values["version"]
	if raw == "" {
		raw = values["version_info"]
	}
	if raw == "" {
		return "", &VersionFileError{Path: path}
	}

	var parts []string
	for _, part := range strings.Split(raw, ".") {
		digits := part[:len(part)-len(strings.TrimLeft(part, "0123456789"))]
		if digits == "" {
			break
		}
		parts = append(parts, digits)
		if len(parts) == 3 || digits != part {
			break
		}
	}
	if len(parts) < 2 {
		return "", &VersionFileError{Path: path, Value: raw}
	}
	return strings.Join(parts, "."), nil
}

// matchVenvVersion returns the installed version to use for a virtual
// environment built from venvVersion: that exact version if it is
// installed, otherwise the highest installed version with the same
// major.minor, since a virtual environment's compiled packages depend on
// the minor version rather than the patch. When nothing installed shares
// the major.minor, it returns the major.minor, so callers report it as not
// installed and suggest installing that line.
func matchVenvVersion(runtimeName, venvVersion string) string {
	installed := installedVersions(runtimeName)
	if slices.Contains(installed, venvVersion) {
		return venvVersion
	}

	parts := strings.Split(venvVersion, ".")
	minor := parts[0] + "." + parts[1]
	if resolved, err := version.ResolvePartialVersion(minor, installed); err == nil {
		return resolved
	}
	return minor
}

// isHomeDir reports whether dir is the user's home directory.
func isHomeDir(dir string) bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	homeInfo, err := os.Stat(home)
	if err != nil {
		return false
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		return false
	}
	return os.SameFile(homeInfo, dirInfo)
}
