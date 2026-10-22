package config

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/version"
)

// versionFiles describes, for each runtime, the files other tools in its
// ecosystem use to pin a project's version.
var versionFiles = map[string]versionFileRules{
	"node":   {names: []string{".node-version", ".nvmrc"}, toolNames: []string{"nodejs", "node"}, prefix: "v"},
	"python": {names: []string{".python-version"}, toolNames: []string{"python"}, prefix: "python-"},
	"ruby":   {names: []string{".ruby-version"}, toolNames: []string{"ruby"}, prefix: "ruby-", prereleases: true},
}

// toolVersionsFile is the file asdf writes and mise reads to pin several
// tools at once, one "<tool> <version>..." line per tool.
const toolVersionsFile = ".tool-versions"

// versionFileRules describes a runtime's version files and how versions are
// written in them.
type versionFileRules struct {
	// names are the runtime's own files, consulted after
	// .dtvem/runtimes.json and .tool-versions in the order listed when
	// several are in one directory.
	names []string
	// toolNames are the names a .tool-versions line may give the runtime,
	// such as asdf's "nodejs" and mise's "node".
	toolNames []string
	// prefix is an optional prefix written before a version, such as the
	// "v" in "v22.12.0" or the "python-" in "python-3.12.4".
	prefix string
	// prereleases allows an exact version to name a pre-release, such as
	// Ruby's "4.0.0-preview2" or "3.4.0-rc1".
	prereleases bool
}

// VersionFileError reports a version file whose contents dtvem cannot use.
type VersionFileError struct {
	Path  string // the version file
	Tool  string // the tool whose line is at fault in a .tool-versions file; "" otherwise
	Value string // the version it asks for; "" when it names none
}

func (e *VersionFileError) Error() string {
	const supported = "dtvem supports only version numbers (x, x.y or x.y.z)"
	switch {
	case e.Tool != "" && e.Value == "":
		return fmt.Sprintf("%s lists %s without a version", e.Path, e.Tool)
	case e.Tool != "":
		return fmt.Sprintf("%s asks for %s %q: %s", e.Path, e.Tool, e.Value, supported)
	case e.Value == "":
		return fmt.Sprintf("%s does not contain a version", e.Path)
	default:
		return fmt.Sprintf("%s asks for %q: %s", e.Path, e.Value, supported)
	}
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

// readToolVersion reads the version a .tool-versions file pins for the
// runtime rules describe. The bool is false when no line names the
// runtime. The error satisfies errors.Is(err, fs.ErrNotExist) when the file
// does not exist.
func readToolVersion(path string, rules versionFileRules) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false, fmt.Errorf("could not read %s: %w", path, err)
	}
	return parseToolVersion(path, string(data), rules)
}

// parseToolVersion finds the runtime's line in the contents of the
// .tool-versions file at path. It skips blank lines and # comments, ignores
// lines for tools dtvem does not manage, and takes the first version on the
// first line that names the runtime; asdf treats any further versions as
// fallbacks. The version must pass the same rules as the runtime's own
// version files.
func parseToolVersion(path, content string, rules versionFileRules) (string, bool, error) {
	for _, line := range strings.Split(content, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 || !slices.Contains(rules.toolNames, fields[0]) {
			continue
		}

		tool := fields[0]
		if len(fields) < 2 {
			return "", true, &VersionFileError{Path: path, Tool: tool}
		}
		spec, err := rules.version(path, fields[1])
		if err != nil {
			return "", true, &VersionFileError{Path: path, Tool: tool, Value: fields[1]}
		}
		return spec, true, nil
	}

	return "", false, nil
}

// version returns the version raw asks for, without the runtime's prefix:
// an exact ("22.12.0") or partial ("22", "22.12") version number, or, if
// the runtime allows them, an exact pre-release ("4.0.0-preview2").
// Anything else, such as an .nvmrc alias like "lts/*" or pyenv's "system",
// is a *VersionFileError for the file at path.
func (r versionFileRules) version(path, raw string) (string, error) {
	spec := strings.TrimPrefix(raw, r.prefix)

	release, pre, isPre := strings.Cut(spec, "-")
	valid := isVersionNumber(release)
	if isPre {
		valid = valid && r.prereleases && strings.Count(release, ".") == 2 && isPrerelease(pre)
	}
	if !valid {
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
		if !isDigits(part) {
			return false
		}
	}
	return true
}

// isPrerelease reports whether s names a pre-release the way Ruby does:
// "preview" or "rc" followed by a number.
func isPrerelease(s string) bool {
	for _, tag := range []string{"preview", "rc"} {
		if n, ok := strings.CutPrefix(s, tag); ok && isDigits(n) {
			return true
		}
	}
	return false
}

// isDigits reports whether s is a non-empty run of ASCII digits.
func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
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
