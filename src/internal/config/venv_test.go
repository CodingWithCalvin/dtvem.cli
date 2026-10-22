package config

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestParseVenvVersion(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "python -m venv",
			content: "home = C:\\Users\\me\\.dtvem\\versions\\python\\3.12.12\ninclude-system-site-packages = false\nversion = 3.12.12\n",
			want:    "3.12.12",
		},
		{name: "uv", content: "home = /home/me/.dtvem/versions/python/3.9.9/bin\nimplementation = CPython\nuv = 0.9.22\nversion_info = 3.9.9\n", want: "3.9.9"},
		{name: "version preferred over version_info", content: "version_info = 3.11.9\nversion = 3.12.4\n", want: "3.12.4"},
		{name: "version_info with release level", content: "version_info = 3.12.1.final.0\n", want: "3.12.1"},
		{name: "pre-release Python", content: "version = 3.13.0rc1\n", want: "3.13.0"},
		{name: "no spaces around =", content: "version=3.12.4", want: "3.12.4"},
		{name: "CRLF", content: "home = C:\\Python312\r\nversion = 3.12.4\r\n", want: "3.12.4"},
		{name: "major.minor only", content: "version = 3.12\n", want: "3.12"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseVenvVersion("pyvenv.cfg", tt.content)
			if err != nil {
				t.Fatalf("parseVenvVersion(%q) unexpected error: %v", tt.content, err)
			}
			if got != tt.want {
				t.Errorf("parseVenvVersion(%q) = %q, want %q", tt.content, got, tt.want)
			}
		})
	}
}

func TestParseVenvVersion_Unusable(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantValue string
	}{
		{name: "empty", content: "", wantValue: ""},
		{name: "no version key", content: "home = /usr/bin\ninclude-system-site-packages = false\n", wantValue: ""},
		{name: "empty version", content: "version = \n", wantValue: ""},
		{name: "not a version", content: "version = unknown\n", wantValue: "unknown"},
		{name: "major only", content: "version = 3\n", wantValue: "3"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseVenvVersion("pyvenv.cfg", tt.content)

			var fileErr *VersionFileError
			if !errors.As(err, &fileErr) {
				t.Fatalf("parseVenvVersion(%q) error = %v, want *VersionFileError", tt.content, err)
			}
			if fileErr.Value != tt.wantValue {
				t.Errorf("VersionFileError.Value = %q, want %q", fileErr.Value, tt.wantValue)
			}
		})
	}
}

func TestResolve_Venv(t *testing.T) {
	tests := []struct {
		name        string
		venvVersion string
		want        string
	}{
		{name: "exact version installed", venvVersion: "3.12.4", want: "3.12.4"},
		{name: "other patch picks highest of the same minor", venvVersion: "3.12.7", want: "3.12.12"},
		{name: "no installed version of the minor", venvVersion: "3.11.9", want: "3.11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVersionFixture(t)
			f.install("python", "3.12.4", "3.12.12", "3.13.1")
			source := f.write(filepath.Join(".venv", "pyvenv.cfg"), "home = /opt/python\nversion = "+tt.venvVersion+"\n")
			f.chdir(filepath.Join("src", "pkg"))

			res, err := Resolve("python")
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			assertResolution(t, res, tt.want, source)
		})
	}
}

func TestResolve_VenvPrecedence(t *testing.T) {
	venv := filepath.Join(".venv", "pyvenv.cfg")

	tests := []struct {
		name    string
		pin     string // file that pins the version beside the .venv
		content string
	}{
		{name: "runtimes.json before .venv", pin: filepath.Join(".dtvem", "runtimes.json"), content: `{"python": "3.13.1"}`},
		{name: ".tool-versions before .venv", pin: ".tool-versions", content: "python 3.13.1\n"},
		{name: ".python-version before .venv", pin: ".python-version", content: "3.13.1\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newVersionFixture(t)
			f.install("python", "3.12.12", "3.13.1")
			f.write(venv, "version = 3.12.12\n")
			source := f.write(tt.pin, tt.content)
			f.chdir(".")

			res, err := Resolve("python")
			if err != nil {
				t.Fatalf("Resolve() error: %v", err)
			}
			assertResolution(t, res, "3.13.1", source)
		})
	}

	t.Run("nearer .venv over parent .python-version", func(t *testing.T) {
		f := newVersionFixture(t)
		f.install("python", "3.12.12", "3.13.1")
		f.write(".python-version", "3.13.1\n")
		source := f.write(filepath.Join("service", ".venv", "pyvenv.cfg"), "version = 3.12.12\n")
		f.chdir("service")

		res, err := Resolve("python")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "3.12.12", source)
	})
}

func TestResolve_VenvPassedOver(t *testing.T) {
	t.Run(".venv without pyvenv.cfg", func(t *testing.T) {
		f := newVersionFixture(t)
		source := f.write(".python-version", "3.13.1\n")
		f.write(filepath.Join("app", ".venv", "Scripts", "python.exe"), "")
		f.chdir("app")

		res, err := Resolve("python")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "3.13.1", source)
	})

	t.Run(".venv that is a file", func(t *testing.T) {
		f := newVersionFixture(t)
		source := f.write(".python-version", "3.13.1\n")
		f.write(filepath.Join("app", ".venv"), "my-virtualenv\n")
		f.chdir("app")

		res, err := Resolve("python")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "3.13.1", source)
	})

	t.Run(".venv in the home directory", func(t *testing.T) {
		f := newVersionFixture(t)
		t.Setenv("HOME", f.project)
		t.Setenv("USERPROFILE", f.project)
		if err := SetGlobalVersion("python", "3.13.1"); err != nil {
			t.Fatalf("SetGlobalVersion() error: %v", err)
		}
		f.write(filepath.Join(".venv", "pyvenv.cfg"), "version = 3.11.9\n")
		f.chdir("code")

		res, err := Resolve("python")
		if err != nil {
			t.Fatalf("Resolve() error: %v", err)
		}
		assertResolution(t, res, "3.13.1", GlobalConfigPath())
	})

	t.Run("other runtimes ignore .venv", func(t *testing.T) {
		f := newVersionFixture(t)
		f.write(filepath.Join(".venv", "pyvenv.cfg"), "version = 3.12.12\n")
		f.chdir(".")

		for _, runtimeName := range []string{"node", "ruby"} {
			if _, err := Resolve(runtimeName); !errors.Is(err, ErrNoVersion) {
				t.Errorf("Resolve(%q) error = %v, want ErrNoVersion", runtimeName, err)
			}
		}
	})
}

func TestResolve_VenvWithoutVersion(t *testing.T) {
	f := newVersionFixture(t)
	if err := SetGlobalVersion("python", "3.13.1"); err != nil {
		t.Fatalf("SetGlobalVersion() error: %v", err)
	}
	f.write(filepath.Join(".venv", "pyvenv.cfg"), "home = /usr/bin\n")
	f.chdir(".")

	var fileErr *VersionFileError
	if _, err := Resolve("python"); !errors.As(err, &fileErr) || filepath.Base(fileErr.Path) != "pyvenv.cfg" {
		t.Errorf("Resolve() error = %v, want a *VersionFileError for pyvenv.cfg", err)
	}
}
