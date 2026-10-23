package doctor

import (
	"errors"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/path"
)

func TestPathRegistryTypeCheck(t *testing.T) {
	expandable := path.RegistryPath{Expandable: true, HasReferences: true}
	unexpanded := path.RegistryPath{Expandable: false, HasReferences: true}
	plain := path.RegistryPath{Expandable: false, HasReferences: false}
	scoped := func(scope string, p path.RegistryPath) path.RegistryPath {
		p.Scope = scope
		return p
	}

	tests := []struct {
		name          string
		paths         []path.RegistryPath
		wantOK        bool
		wantFixable   bool
		wantDetails   []string
		wantInResolve []string
	}{
		{
			name:   "not Windows",
			paths:  nil,
			wantOK: true,
		},
		{
			name:   "both expandable",
			paths:  []path.RegistryPath{scoped("User", expandable), scoped("System", expandable)},
			wantOK: true,
		},
		{
			name:   "REG_SZ without references is fine",
			paths:  []path.RegistryPath{scoped("User", plain), scoped("System", expandable)},
			wantOK: true,
		},
		{
			name:          "User PATH unexpanded",
			paths:         []path.RegistryPath{scoped("User", unexpanded), scoped("System", expandable)},
			wantFixable:   true,
			wantDetails:   []string{"User PATH"},
			wantInResolve: []string{"Rewrite the User PATH as REG_EXPAND_SZ"},
		},
		{
			name:          "System PATH unexpanded",
			paths:         []path.RegistryPath{scoped("User", expandable), scoped("System", unexpanded)},
			wantFixable:   false,
			wantDetails:   []string{"System PATH"},
			wantInResolve: []string{"elevated PowerShell", "'ExpandString'"},
		},
		{
			name:          "both unexpanded",
			paths:         []path.RegistryPath{scoped("User", unexpanded), scoped("System", unexpanded)},
			wantFixable:   true,
			wantDetails:   []string{"User PATH", "System PATH"},
			wantInResolve: []string{"Rewrite the User PATH", "elevated PowerShell"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := pathRegistryTypeCheck{
				readPaths:  func() ([]path.RegistryPath, error) { return tt.paths, nil },
				repairUser: func() error { return nil },
			}

			f := c.Run()
			if f.OK != tt.wantOK {
				t.Fatalf("Run().OK = %v, want %v (%+v)", f.OK, tt.wantOK, f)
			}
			if tt.wantOK {
				return
			}
			if f.Severity != SeverityWarning {
				t.Errorf("Severity = %v, want warning", f.Severity)
			}
			if f.Fixable() != tt.wantFixable {
				t.Errorf("Fixable() = %v, want %v", f.Fixable(), tt.wantFixable)
			}
			var gotDetails []string
			for _, d := range f.Details {
				gotDetails = append(gotDetails, d.Key)
			}
			if strings.Join(gotDetails, ",") != strings.Join(tt.wantDetails, ",") {
				t.Errorf("Details = %v, want %v", gotDetails, tt.wantDetails)
			}
			for _, s := range tt.wantInResolve {
				if !strings.Contains(f.Resolution, s) {
					t.Errorf("Resolution %q does not mention %q", f.Resolution, s)
				}
			}
		})
	}
}

func TestPathRegistryTypeCheck_FixRepairsUserPath(t *testing.T) {
	repaired := false
	c := pathRegistryTypeCheck{
		readPaths: func() ([]path.RegistryPath, error) {
			return []path.RegistryPath{{Scope: "User", HasReferences: true}}, nil
		},
		repairUser: func() error { repaired = true; return nil },
	}

	f := c.Run()
	if !f.Fixable() {
		t.Fatal("finding is not fixable")
	}
	if err := f.Fix(); err != nil {
		t.Fatalf("Fix() error: %v", err)
	}
	if !repaired {
		t.Error("Fix() did not repair the User PATH")
	}
}

func TestPathRegistryTypeCheck_ReadError(t *testing.T) {
	c := pathRegistryTypeCheck{
		readPaths: func() ([]path.RegistryPath, error) { return nil, errors.New("access denied") },
	}

	f := c.Run()
	if f.OK || f.Severity != SeverityWarning || f.Fixable() {
		t.Errorf("Run() = %+v, want an unfixable warning", f)
	}
}

func TestPathRegistryTypeCheck_Registered(t *testing.T) {
	for _, c := range All() {
		if c.Name() == "path-registry-type" {
			return
		}
	}
	t.Error("path-registry-type check is not registered")
}
