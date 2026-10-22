package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestParseVersionSpec(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "exact version", content: "22.12.0", want: "22.12.0"},
		{name: "exact version with v prefix", content: "v22.12.0", want: "22.12.0"},
		{name: "major only", content: "22", want: "22"},
		{name: "major and minor", content: "22.12", want: "22.12"},
		{name: "partial with v prefix", content: "v22", want: "22"},
		{name: "trailing newline", content: "22.12.0\n", want: "22.12.0"},
		{name: "CRLF line endings", content: "22.12.0\r\n", want: "22.12.0"},
		{name: "surrounding whitespace", content: "  \t22.12.0  \n", want: "22.12.0"},
		{name: "leading blank lines", content: "\n\n  \n22\n", want: "22"},
		{name: "comment line before version", content: "# pinned for CI\n22.12.0\n", want: "22.12.0"},
		{name: "inline comment", content: "22 # current LTS\n", want: "22"},
		{name: "first version line wins", content: "22.12.0\n20.0.0\n", want: "22.12.0"},
		{name: "first word of the line wins", content: "22.12.0 20.0.0\n", want: "22.12.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionSpec(".node-version", tt.content, versionFiles["node"])
			if err != nil {
				t.Fatalf("parseVersionSpec(%q) unexpected error: %v", tt.content, err)
			}
			if got != tt.want {
				t.Errorf("parseVersionSpec(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestParseVersionSpec_Unsupported(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantValue string
	}{
		{name: "empty file", content: "", wantValue: ""},
		{name: "whitespace only", content: "  \n\t\n", wantValue: ""},
		{name: "comment only", content: "# no version here\n", wantValue: ""},
		{name: "lts alias", content: "lts/*\n", wantValue: "lts/*"},
		{name: "named lts alias", content: "lts/jod", wantValue: "lts/jod"},
		{name: "node alias", content: "node", wantValue: "node"},
		{name: "wildcard minor", content: "22.x", wantValue: "22.x"},
		{name: "too many components", content: "22.12.0.1", wantValue: "22.12.0.1"},
		{name: "empty component", content: "22..1", wantValue: "22..1"},
		{name: "bare v", content: "v", wantValue: "v"},
		{name: "prerelease", content: "23.0.0-rc.1", wantValue: "23.0.0-rc.1"},
		{name: "another runtime's prefix", content: "python-22", wantValue: "python-22"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVersionSpec(".nvmrc", tt.content, versionFiles["node"])

			var fileErr *VersionFileError
			if !errors.As(err, &fileErr) {
				t.Fatalf("parseVersionSpec(%q) error = %v, want *VersionFileError", tt.content, err)
			}
			if fileErr.Path != ".nvmrc" {
				t.Errorf("VersionFileError.Path = %q, want %q", fileErr.Path, ".nvmrc")
			}
			if fileErr.Value != tt.wantValue {
				t.Errorf("VersionFileError.Value = %q, want %q", fileErr.Value, tt.wantValue)
			}
		})
	}
}

func TestVersionFileError_Message(t *testing.T) {
	alias := (&VersionFileError{Path: ".nvmrc", Value: "lts/*"}).Error()
	if want := `.nvmrc asks for "lts/*": dtvem supports only version numbers (x, x.y or x.y.z)`; alias != want {
		t.Errorf("Error() = %q, want %q", alias, want)
	}

	empty := (&VersionFileError{Path: ".node-version"}).Error()
	if want := ".node-version does not contain a version"; empty != want {
		t.Errorf("Error() = %q, want %q", empty, want)
	}
}

func TestParseVersionSpec_Python(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "exact version", content: "3.12.4\n", want: "3.12.4"},
		{name: "partial version", content: "3.12", want: "3.12"},
		{name: "python- prefix", content: "python-3.12.4", want: "3.12.4"},
		{name: "comment and blank lines", content: "# pinned by uv\n\n3.12.4\n", want: "3.12.4"},
		{name: "first of several versions", content: "3.12.4\n3.11.9\n", want: "3.12.4"},
		{name: "CRLF line endings", content: "3.12.4\r\n", want: "3.12.4"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionSpec(".python-version", tt.content, versionFiles["python"])
			if err != nil {
				t.Fatalf("parseVersionSpec(%q) unexpected error: %v", tt.content, err)
			}
			if got != tt.want {
				t.Errorf("parseVersionSpec(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestParseVersionSpec_PythonUnsupported(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantValue string
	}{
		{name: "system", content: "system\n", wantValue: "system"},
		{name: "pyenv-virtualenv pointer", content: "3.10/envs/venv", wantValue: "3.10/envs/venv"},
		{name: "virtualenv name", content: "my-venv", wantValue: "my-venv"},
		{name: "uv implementation request", content: "pypy", wantValue: "pypy"},
		{name: "uv implementation@version request", content: "cpython@3.12", wantValue: "cpython@3.12"},
		{name: "v prefix is not Python's", content: "v3.12.4", wantValue: "v3.12.4"},
		{name: "empty file", content: "\n", wantValue: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVersionSpec(".python-version", tt.content, versionFiles["python"])

			var fileErr *VersionFileError
			if !errors.As(err, &fileErr) {
				t.Fatalf("parseVersionSpec(%q) error = %v, want *VersionFileError", tt.content, err)
			}
			if fileErr.Value != tt.wantValue {
				t.Errorf("VersionFileError.Value = %q, want %q", fileErr.Value, tt.wantValue)
			}
		})
	}
}

func TestParseVersionSpec_Ruby(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{name: "exact version", content: "3.3.6\n", want: "3.3.6"},
		{name: "ruby- prefix", content: "ruby-3.3.6", want: "3.3.6"},
		{name: "partial version", content: "3.3", want: "3.3"},
		{name: "preview", content: "4.0.0-preview2", want: "4.0.0-preview2"},
		{name: "release candidate with ruby- prefix", content: "ruby-3.4.0-rc1", want: "3.4.0-rc1"},
		{name: "comment and blank lines", content: "# CI pin\n\n3.3.6\n", want: "3.3.6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVersionSpec(".ruby-version", tt.content, versionFiles["ruby"])
			if err != nil {
				t.Fatalf("parseVersionSpec(%q) unexpected error: %v", tt.content, err)
			}
			if got != tt.want {
				t.Errorf("parseVersionSpec(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestParseVersionSpec_RubyUnsupported(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{name: "JRuby", content: "jruby-9.4.5.0"},
		{name: "TruffleRuby", content: "truffleruby-24.1.0"},
		{name: "engine-only ruby", content: "ruby"},
		{name: "engine-only truffleruby", content: "truffleruby"},
		{name: "system", content: "system"},
		{name: "partial pre-release", content: "4.0-preview2"},
		{name: "unknown pre-release tag", content: "3.4.0-beta1"},
		{name: "pre-release without a number", content: "3.4.0-preview"},
		{name: "old patch level", content: "2.0.0-p648"},
		{name: "v prefix is not Ruby's", content: "v3.3.6"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVersionSpec(".ruby-version", tt.content, versionFiles["ruby"])

			var fileErr *VersionFileError
			if !errors.As(err, &fileErr) {
				t.Fatalf("parseVersionSpec(%q) error = %v, want *VersionFileError", tt.content, err)
			}
			if fileErr.Value != tt.content {
				t.Errorf("VersionFileError.Value = %q, want %q", fileErr.Value, tt.content)
			}
		})
	}
}

func TestParseVersionSpec_PrereleasesAreRubyOnly(t *testing.T) {
	for _, runtimeName := range []string{"node", "python"} {
		_, err := parseVersionSpec("version-file", "4.0.0-preview2", versionFiles[runtimeName])

		var fileErr *VersionFileError
		if !errors.As(err, &fileErr) {
			t.Errorf("parseVersionSpec(%s rules, pre-release) error = %v, want *VersionFileError", runtimeName, err)
		}
	}
}

// versionFixture is a dtvem root and a project directory to resolve from.
type versionFixture struct {
	t       *testing.T
	root    string // DTVEM_ROOT
	project string // a directory outside the dtvem root
}

// newVersionFixture points DTVEM_ROOT at a fresh directory and creates an
// empty project directory beside it. The working directory is restored when
// the test ends.
func newVersionFixture(t *testing.T) *versionFixture {
	t.Helper()

	tmp := t.TempDir()
	f := &versionFixture{
		t:       t,
		root:    filepath.Join(tmp, "dtvem"),
		project: filepath.Join(tmp, "project"),
	}
	if err := os.MkdirAll(f.project, 0755); err != nil {
		t.Fatalf("Failed to create project directory: %v", err)
	}

	t.Setenv("DTVEM_ROOT", f.root)
	ResetPathsCache()
	t.Cleanup(ResetPathsCache)

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(originalDir) })

	return f
}

// write creates a file under the project directory.
func (f *versionFixture) write(rel, content string) string {
	f.t.Helper()
	path := filepath.Join(f.project, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		f.t.Fatalf("Failed to create directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		f.t.Fatalf("Failed to write %s: %v", rel, err)
	}
	return path
}

// install creates empty installed-version directories for a runtime.
func (f *versionFixture) install(runtimeName string, versions ...string) {
	f.t.Helper()
	for _, v := range versions {
		if err := os.MkdirAll(filepath.Join(f.root, "versions", runtimeName, v), 0755); err != nil {
			f.t.Fatalf("Failed to create installed version %s: %v", v, err)
		}
	}
}

// chdir changes into a directory under the project, creating it if needed.
func (f *versionFixture) chdir(rel string) {
	f.t.Helper()
	dir := filepath.Join(f.project, rel)
	if err := os.MkdirAll(dir, 0755); err != nil {
		f.t.Fatalf("Failed to create %s: %v", rel, err)
	}
	if err := os.Chdir(dir); err != nil {
		f.t.Fatalf("Failed to change directory: %v", err)
	}
}

// assertResolution checks a Resolution against the expected version and
// source file, comparing paths with symlinks resolved (macOS reports
// /var/... temp directories as /private/var/...).
func assertResolution(t *testing.T, got Resolution, wantVersion, wantSource string) {
	t.Helper()
	if got.Version != wantVersion {
		t.Errorf("Version = %q, want %q", got.Version, wantVersion)
	}
	gotSource, err := filepath.EvalSymlinks(got.Source)
	if err != nil {
		t.Fatalf("Source %q does not exist: %v", got.Source, err)
	}
	wantSource, err = filepath.EvalSymlinks(wantSource)
	if err != nil {
		t.Fatalf("expected source %q does not exist: %v", wantSource, err)
	}
	if gotSource != wantSource {
		t.Errorf("Source = %q, want %q", gotSource, wantSource)
	}
}

func TestResolve_NodeVersionFile(t *testing.T) {
	f := newVersionFixture(t)
	source := f.write(".node-version", "22.12.0\n")
	f.chdir(".")

	res, err := Resolve("node")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	assertResolution(t, res, "22.12.0", source)
}

func TestResolve_NvmrcFile(t *testing.T) {
	f := newVersionFixture(t)
	source := f.write(".nvmrc", "v20.11.1\n")
	f.chdir(".")

	res, err := Resolve("node")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	assertResolution(t, res, "20.11.1", source)
}

func TestResolve_SubdirectoryInheritsVersionFile(t *testing.T) {
	f := newVersionFixture(t)
	source := f.write(".node-version", "22.12.0")
	f.chdir(filepath.Join("packages", "app", "src"))

	res, err := Resolve("node")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	assertResolution(t, res, "22.12.0", source)
}

func TestResolve_PrecedenceWithinDirectory(t *testing.T) {
	t.Run("runtimes.json before .node-version", func(t *testing.T) {
		f := newVersionFixture(t)
		source := f.write(filepath.Join(".dtvem", "runtimes.json"), `{"node": "20.0.0"}`)
		f.write(".node-version", "22.12.0")
		f.write(".nvmrc", "18.0.0")
		f.chdir(".")

		res, err := Resolve("node")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "20.0.0", source)
	})

	t.Run(".node-version before .nvmrc", func(t *testing.T) {
		f := newVersionFixture(t)
		source := f.write(".node-version", "22.12.0")
		f.write(".nvmrc", "18.0.0")
		f.chdir(".")

		res, err := Resolve("node")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "22.12.0", source)
	})

	t.Run("runtimes.json without node falls through to .node-version", func(t *testing.T) {
		f := newVersionFixture(t)
		f.write(filepath.Join(".dtvem", "runtimes.json"), `{"python": "3.12.0"}`)
		source := f.write(".node-version", "22.12.0")
		f.chdir(".")

		res, err := Resolve("node")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "22.12.0", source)
	})
}

func TestResolve_NearestDirectoryWins(t *testing.T) {
	t.Run("version file in child over runtimes.json in parent", func(t *testing.T) {
		f := newVersionFixture(t)
		f.write(filepath.Join(".dtvem", "runtimes.json"), `{"node": "20.0.0"}`)
		source := f.write(filepath.Join("app", ".node-version"), "22.12.0")
		f.chdir("app")

		res, err := Resolve("node")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "22.12.0", source)
	})

	t.Run("runtimes.json in child over version file in parent", func(t *testing.T) {
		f := newVersionFixture(t)
		f.write(".nvmrc", "22.12.0")
		source := f.write(filepath.Join("app", ".dtvem", "runtimes.json"), `{"node": "20.0.0"}`)
		f.chdir("app")

		res, err := Resolve("node")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "20.0.0", source)
	})
}

func TestResolve_PartialVersion(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want string
	}{
		{name: "major picks highest installed", spec: "22", want: "22.12.0"},
		{name: "major.minor picks highest patch", spec: "22.9", want: "22.9.1"},
		{name: "v prefix", spec: "v20", want: "20.18.0"},
		{name: "numeric not lexical match", spec: "2", want: "2"},
		{name: "no installed match stays partial", spec: "21", want: "21"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVersionFixture(t)
			f.install("node", "22.1.0", "22.9.0", "22.9.1", "22.12.0", "20.18.0", "220.0.0")
			source := f.write(".node-version", tt.spec)
			f.chdir(".")

			res, err := Resolve("node")
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			assertResolution(t, res, tt.want, source)
		})
	}
}

func TestResolve_ExactVersionNotInstalled(t *testing.T) {
	f := newVersionFixture(t)
	f.install("node", "22.12.0")
	source := f.write(".node-version", "22.0.0")
	f.chdir(".")

	res, err := Resolve("node")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	assertResolution(t, res, "22.0.0", source)
}

func TestResolve_UnsupportedVersionFileDoesNotFallBack(t *testing.T) {
	f := newVersionFixture(t)
	if err := SetGlobalVersion("node", "22.12.0"); err != nil {
		t.Fatalf("SetGlobalVersion() error: %v", err)
	}
	// A valid pin further up must not be used either.
	f.write(".node-version", "20.0.0")
	f.write(filepath.Join("app", ".nvmrc"), "lts/*\n")
	f.chdir("app")

	_, err := Resolve("node")

	var fileErr *VersionFileError
	if !errors.As(err, &fileErr) {
		t.Fatalf("Resolve() error = %v, want *VersionFileError", err)
	}
	if fileErr.Value != "lts/*" {
		t.Errorf("VersionFileError.Value = %q, want %q", fileErr.Value, "lts/*")
	}
	if filepath.Base(fileErr.Path) != ".nvmrc" {
		t.Errorf("VersionFileError.Path = %q, want an .nvmrc path", fileErr.Path)
	}
	if errors.Is(err, ErrNoVersion) {
		t.Error("Resolve() error should not wrap ErrNoVersion for an unsupported version file")
	}
}

func TestResolve_EmptyVersionFile(t *testing.T) {
	f := newVersionFixture(t)
	f.write(".node-version", "\n")
	f.chdir(".")

	_, err := Resolve("node")

	var fileErr *VersionFileError
	if !errors.As(err, &fileErr) {
		t.Fatalf("Resolve() error = %v, want *VersionFileError", err)
	}
	if fileErr.Value != "" {
		t.Errorf("VersionFileError.Value = %q, want empty", fileErr.Value)
	}
}

func TestResolve_GlobalFallback(t *testing.T) {
	f := newVersionFixture(t)
	if err := SetGlobalVersion("node", "22.12.0"); err != nil {
		t.Fatalf("SetGlobalVersion() error: %v", err)
	}
	f.chdir(".")

	res, err := Resolve("node")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	assertResolution(t, res, "22.12.0", GlobalConfigPath())
}

func TestResolve_NoVersion(t *testing.T) {
	f := newVersionFixture(t)
	f.chdir(".")

	_, err := Resolve("node")
	if !errors.Is(err, ErrNoVersion) {
		t.Errorf("Resolve() error = %v, want ErrNoVersion", err)
	}
}

func TestResolve_VersionFilesAreRuntimeSpecific(t *testing.T) {
	f := newVersionFixture(t)
	f.write(".node-version", "22.12.0")
	f.write(".nvmrc", "lts/*")
	f.write(".python-version", "system")
	f.chdir(".")

	if _, err := Resolve("ruby"); !errors.Is(err, ErrNoVersion) {
		t.Errorf("Resolve(\"ruby\") error = %v, want ErrNoVersion", err)
	}

	res, err := Resolve("node")
	if err != nil {
		t.Fatalf("Resolve(\"node\") error: %v", err)
	}
	if res.Version != "22.12.0" {
		t.Errorf("Resolve(\"node\") = %q, want %q", res.Version, "22.12.0")
	}

	var fileErr *VersionFileError
	if _, err := Resolve("python"); !errors.As(err, &fileErr) || filepath.Base(fileErr.Path) != ".python-version" {
		t.Errorf("Resolve(\"python\") error = %v, want a *VersionFileError for .python-version", err)
	}
}

func TestResolve_PythonVersionFile(t *testing.T) {
	f := newVersionFixture(t)
	f.install("python", "3.11.9", "3.12.4", "3.12.12", "3.13.1")
	source := f.write(".python-version", "3.12\n")
	f.chdir(filepath.Join("src", "pkg"))

	res, err := Resolve("python")
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	assertResolution(t, res, "3.12.12", source)
}

func TestResolve_RubyVersionFile(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want string
	}{
		{name: "exact", spec: "3.3.9", want: "3.3.9"},
		{name: "ruby- prefix", spec: "ruby-3.3.9", want: "3.3.9"},
		{name: "partial picks highest installed", spec: "3.4", want: "3.4.7"},
		{name: "partial picks release over its preview", spec: "4.0", want: "4.0.0"},
		{name: "major picks release over its preview", spec: "4", want: "4.0.0"},
		{name: "partial with only a preview installed", spec: "3.5", want: "3.5.0-preview1"},
		{name: "exact preview", spec: "4.0.0-preview2", want: "4.0.0-preview2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVersionFixture(t)
			f.install("ruby", "3.3.9", "3.4.1", "3.4.7", "3.5.0-preview1", "4.0.0-preview2", "4.0.0")
			source := f.write(".ruby-version", tt.spec+"\n")
			f.chdir("lib")

			res, err := Resolve("ruby")
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			assertResolution(t, res, tt.want, source)
		})
	}
}

func TestResolve_RubyPrecedence(t *testing.T) {
	t.Run("runtimes.json before .ruby-version", func(t *testing.T) {
		f := newVersionFixture(t)
		source := f.write(filepath.Join(".dtvem", "runtimes.json"), `{"ruby": "3.3.9"}`)
		f.write(".ruby-version", "3.4.7")
		f.chdir(".")

		res, err := Resolve("ruby")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "3.3.9", source)
	})

	t.Run("unsupported .ruby-version does not fall back to the global version", func(t *testing.T) {
		f := newVersionFixture(t)
		if err := SetGlobalVersion("ruby", "3.4.7"); err != nil {
			t.Fatalf("SetGlobalVersion() error: %v", err)
		}
		f.write(".ruby-version", "jruby-9.4.5.0")
		f.chdir(".")

		var fileErr *VersionFileError
		if _, err := Resolve("ruby"); !errors.As(err, &fileErr) {
			t.Errorf("Resolve() error = %v, want *VersionFileError", err)
		}
	})
}

func TestResolve_PythonPrecedence(t *testing.T) {
	t.Run("runtimes.json before .python-version", func(t *testing.T) {
		f := newVersionFixture(t)
		source := f.write(filepath.Join(".dtvem", "runtimes.json"), `{"python": "3.11.9"}`)
		f.write(".python-version", "3.12.4")
		f.chdir(".")

		res, err := Resolve("python")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "3.11.9", source)
	})

	t.Run("nearer .python-version over parent runtimes.json", func(t *testing.T) {
		f := newVersionFixture(t)
		f.write(filepath.Join(".dtvem", "runtimes.json"), `{"python": "3.11.9"}`)
		source := f.write(filepath.Join("app", ".python-version"), "python-3.12.4")
		f.chdir("app")

		res, err := Resolve("python")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "3.12.4", source)
	})
}

func TestResolveVersion_UsesVersionFiles(t *testing.T) {
	f := newVersionFixture(t)
	f.install("node", "22.12.0")
	f.write(".nvmrc", "22")
	f.chdir(".")

	got, err := ResolveVersion("node")
	if err != nil {
		t.Fatalf("ResolveVersion() error: %v", err)
	}
	if got != "22.12.0" {
		t.Errorf("ResolveVersion() = %q, want %q", got, "22.12.0")
	}
}

func TestLocalVersion_UsesVersionFiles(t *testing.T) {
	f := newVersionFixture(t)
	if err := SetGlobalVersion("node", "20.0.0"); err != nil {
		t.Fatalf("SetGlobalVersion() error: %v", err)
	}
	f.write(".node-version", "22.12.0")
	f.chdir(".")

	got, err := LocalVersion("node")
	if err != nil {
		t.Fatalf("LocalVersion() error: %v", err)
	}
	if got != "22.12.0" {
		t.Errorf("LocalVersion() = %q, want %q", got, "22.12.0")
	}
}

func TestLocalVersion_IgnoresGlobal(t *testing.T) {
	f := newVersionFixture(t)
	if err := SetGlobalVersion("node", "20.0.0"); err != nil {
		t.Fatalf("SetGlobalVersion() error: %v", err)
	}
	f.chdir(".")

	if got, err := LocalVersion("node"); err == nil {
		t.Errorf("LocalVersion() = %q, want an error when only a global version is set", got)
	}
}
