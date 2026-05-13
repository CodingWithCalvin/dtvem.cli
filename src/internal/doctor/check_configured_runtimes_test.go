package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// installAwareProvider is a fakeProvider variant that tracks
// IsInstalled answers per (name, version) pair. The runtime registry
// can't be cleanly populated in tests, so we route through the
// check's injected getProvider closure instead.
type installAwareProvider struct {
	*fakeProvider
	installed map[string]bool
	checkErr  error
}

func (p *installAwareProvider) IsInstalled(version string) (bool, error) {
	if p.checkErr != nil {
		return false, p.checkErr
	}
	return p.installed[version], nil
}

// configuredCheckWith returns a check wired with a synthetic config
// and provider lookup. Providers are keyed by runtime name; missing
// keys fall through to "unknown runtime" handling.
func configuredCheckWith(cfg config.RuntimesConfig, providers map[string]*installAwareProvider, cfgErr error) *configuredRuntimesCheck {
	c := newConfiguredRuntimesCheck()
	c.configPath = func() string { return "/tmp/runtimes.json" }
	c.readConfig = func(_ string) (config.RuntimesConfig, error) { return cfg, cfgErr }
	c.getProvider = func(name string) (runtime.ShimProvider, error) {
		p, ok := providers[name]
		if !ok {
			return nil, errors.New("provider not found: " + name)
		}
		return p, nil
	}
	return c
}

func newInstallAwareProvider(name, display string, installed map[string]bool) *installAwareProvider {
	return &installAwareProvider{
		fakeProvider: &fakeProvider{name: name, displayName: display, shims: []string{name}},
		installed:    installed,
	}
}

func TestConfiguredRuntimesCheck_AllInstalledIsOK(t *testing.T) {
	cfg := config.RuntimesConfig{"python": "3.11.0", "node": "22.0.0"}
	providers := map[string]*installAwareProvider{
		"python": newInstallAwareProvider("python", "Python", map[string]bool{"3.11.0": true}),
		"node":   newInstallAwareProvider("node", "Node.js", map[string]bool{"22.0.0": true}),
	}
	got := configuredCheckWith(cfg, providers, nil).Run()
	if !got.OK {
		t.Errorf("expected OK when all configured versions are installed, got %#v", got)
	}
}

func TestConfiguredRuntimesCheck_FlagsMissingInstall(t *testing.T) {
	cfg := config.RuntimesConfig{"python": "3.11.0"}
	providers := map[string]*installAwareProvider{
		"python": newInstallAwareProvider("python", "Python", map[string]bool{}),
	}
	got := configuredCheckWith(cfg, providers, nil).Run()
	if got.OK {
		t.Fatalf("expected non-OK when configured version is missing, got OK")
	}
	if got.Severity != SeverityError {
		t.Errorf("severity: got %s, want error", got.Severity)
	}
	if got.Fixable() {
		t.Errorf("configured-runtimes check should be manual")
	}

	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, "3.11.0") {
		t.Errorf("expected version 3.11.0 in details, got %#v", got.Details)
	}
	if !strings.Contains(got.Resolution, "dtvem install") {
		t.Errorf("resolution should suggest `dtvem install`, got %q", got.Resolution)
	}
}

func TestConfiguredRuntimesCheck_UnknownRuntimeIsFlagged(t *testing.T) {
	cfg := config.RuntimesConfig{"madeup": "1.2.3"}
	got := configuredCheckWith(cfg, map[string]*installAwareProvider{}, nil).Run()
	if got.OK {
		t.Fatalf("expected non-OK when config references unknown runtime, got OK")
	}
	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, "unknown") {
		t.Errorf("expected detail mentioning 'unknown', got %#v", got.Details)
	}
}

func TestConfiguredRuntimesCheck_IsInstalledErrorIsFlagged(t *testing.T) {
	cfg := config.RuntimesConfig{"python": "3.11.0"}
	failing := newInstallAwareProvider("python", "Python", nil)
	failing.checkErr = errors.New("disk read failed")
	providers := map[string]*installAwareProvider{"python": failing}

	got := configuredCheckWith(cfg, providers, nil).Run()
	if got.OK {
		t.Fatalf("expected non-OK when IsInstalled errors, got OK")
	}
	combined := strings.Join(detailValues(got.Details), " ")
	if !strings.Contains(combined, "disk read failed") {
		t.Errorf("expected error text in detail, got %#v", got.Details)
	}
}

func TestConfiguredRuntimesCheck_MissingConfigFileIsOK(t *testing.T) {
	missing := &os.PathError{Op: "open", Path: "/tmp/runtimes.json", Err: os.ErrNotExist}
	got := configuredCheckWith(nil, nil, missing).Run()
	if !got.OK {
		t.Errorf("expected OK when config file is missing, got %#v", got)
	}
}

func TestConfiguredRuntimesCheck_UnreadableConfigIsWarning(t *testing.T) {
	got := configuredCheckWith(nil, nil, errors.New("corrupt json")).Run()
	if got.OK {
		t.Fatalf("expected non-OK when config can't be read, got OK")
	}
	if got.Severity != SeverityWarning {
		t.Errorf("severity: got %s, want warning", got.Severity)
	}
}

func TestConfiguredRuntimesCheck_EmptyConfigIsOK(t *testing.T) {
	got := configuredCheckWith(config.RuntimesConfig{}, nil, nil).Run()
	if !got.OK {
		t.Errorf("expected OK when config is empty, got %#v", got)
	}
}

func TestConfiguredRuntimesCheck_DetailsAreSorted(t *testing.T) {
	cfg := config.RuntimesConfig{
		"zruntime": "1.0.0",
		"aruntime": "1.0.0",
	}
	providers := map[string]*installAwareProvider{
		"zruntime": newInstallAwareProvider("zruntime", "Z", map[string]bool{}),
		"aruntime": newInstallAwareProvider("aruntime", "A", map[string]bool{}),
	}
	got := configuredCheckWith(cfg, providers, nil).Run()

	// Details start with "Config" then the per-runtime entries
	// sorted by name. Confirm "A" precedes "Z".
	idxA, idxZ := -1, -1
	for i, d := range got.Details {
		if d.Key == "A" {
			idxA = i
		}
		if d.Key == "Z" {
			idxZ = i
		}
	}
	if idxA < 0 || idxZ < 0 || idxA > idxZ {
		t.Errorf("expected A detail before Z detail; details: %#v", got.Details)
	}
}

func TestConfiguredRuntimesCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "configured-runtimes-installed" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("configured-runtimes-installed check is not in the default registry")
	}
}

// Touch a config file constant just to make sure config import works
// in this test file's package — keeps lints happy when test bodies
// don't otherwise reference imported packages.
var _ = filepath.Join
