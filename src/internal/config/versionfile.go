package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/version"
)

// versionFiles describes, for each runtime, the files other tools in its
// ecosystem use to pin a project's version.
var versionFiles = map[string]versionFileRules{
	"node":   {names: []string{".node-version", ".nvmrc"}, prefix: "v"},
	"python": {names: []string{".python-version"}, prefix: "python-"},
}

// versionFileRules describes a runtime's version files and how versions are
// written in them.
type versionFileRules struct {
	// names are the files, consulted after .dtvem/runtimes.json in the
	// order listed when several are in one directory.
	names []string
	// prefix is an optional prefix written before a version, such as the
	// "v" in "v22.12.0" or the "python-" in "python-3.12.4".
	prefix string
}

// VersionFileError reports a version file whose contents dtvem cannot use.
type VersionFileError struct {
	Path  string // the version file
	Value string // the version it asks for; "" when it names none
}

func (e *VersionFileError) Error() string {
	if e.Value == "" {
		return fmt.Sprintf("%s does not contain a version", e.Path)
	}
	return fmt.Sprintf("%s asks for %q: dtvem supports only version numbers (x, x.y or x.y.z)", e.Path, e.Value)
}

// readVersionSpec reads a version file such as .node-version or
// .python-version and returns the version it asks for. The error satisfies
// errors.Is(err, fs.ErrNotExist) when the file does not exist.
func readVersionSpec(path string, rules versionFileRules) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("could not read %s: %w", path, err)
	}
	return parseVersionSpec(path, string(data), rules)
}

// parseVersionSpec extracts the version from the contents of the version
// file at path. It skips blank lines and # comments and takes the first
// word of the first remaining line, as pyenv and rbenv do. See
// versionFileRules.version for the forms accepted.
func parseVersionSpec(path, content string, rules versionFileRules) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			return rules.version(path, fields[0])
		}
	}

	return "", &VersionFileError{Path: path}
}

// version returns the version raw asks for, without the runtime's prefix:
// an exact ("22.12.0") or partial ("22", "22.12") version number. Anything
// else, such as an .nvmrc alias like "lts/*" or pyenv's "system", is a
// *VersionFileError for the file at path.
func (r versionFileRules) version(path, raw string) (string, error) {
	spec := strings.TrimPrefix(raw, r.prefix)
	if !isVersionNumber(spec) {
		return "", &VersionFileError{Path: path, Value: raw}
	}
	return spec, nil
}

// isVersionNumber reports whether s is one to three dot-separated runs of
// digits.
func isVersionNumber(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) > 3 {
		return false
	}
	for _, part := range parts {
		if part == "" {
			return false
		}
		for _, c := range part {
			if c < '0' || c > '9' {
				return false
			}
		}
	}
	return true
}

// resolveSpec returns the version a version file's spec selects: the spec
// itself when it is exact, or the highest installed version it matches when
// it is partial. A partial spec that matches nothing installed is returned
// unchanged, so callers report it as not installed.
func resolveSpec(runtimeName, spec string) string {
	if !version.IsPartialVersion(spec) {
		return spec
	}

	resolved, err := version.ResolvePartialVersion(spec, installedVersions(runtimeName))
	if err != nil {
		return spec
	}
	return resolved
}

// installedVersions returns the names of the installed versions of a
// runtime, or nil if none are installed.
func installedVersions(runtimeName string) []string {
	entries, err := os.ReadDir(filepath.Join(DefaultPaths().Versions, runtimeName))
	if err != nil {
		return nil
	}

	var versions []string
	for _, entry := range entries {
		if entry.IsDir() {
			versions = append(versions, entry.Name())
		}
	}
	return versions
}
