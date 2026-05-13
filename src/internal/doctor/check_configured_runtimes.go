package doctor

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// configuredRuntimesCheck verifies that every runtime/version pair in
// the global ~/.dtvem/config/runtimes.json points at an installed
// version. A mismatch means `dtvem current` will report a version
// dtvem can't actually execute, and shim invocations for that runtime
// will fail with a confusing "version not installed" error rather
// than a useful one at config-load time.
//
// The fix isn't automatable in general — doctor can't tell whether
// the user wants to install the configured version or edit the
// config to match what's installed — so this is a manual check with
// a one-line `dtvem install` suggestion per mismatch.
type configuredRuntimesCheck struct {
	configPath  func() string
	readConfig  func(path string) (config.RuntimesConfig, error)
	getProvider func(name string) (runtime.ShimProvider, error)
}

func newConfiguredRuntimesCheck() *configuredRuntimesCheck {
	return &configuredRuntimesCheck{
		configPath:  config.GlobalConfigPath,
		readConfig:  config.ReadAllRuntimes,
		getProvider: runtime.GetShimProvider,
	}
}

func (configuredRuntimesCheck) Name() string { return "configured-runtimes-installed" }

func (c configuredRuntimesCheck) Run() Finding {
	cfgPath := c.configPath()
	cfg, err := c.readConfig(cfgPath)
	if err != nil {
		if os.IsNotExist(err) {
			return Finding{OK: true, Title: "No global runtimes config to check"}
		}
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not read global runtimes config",
			Details:    []Detail{{Key: "Path", Value: cfgPath}, {Key: "Error", Value: err.Error()}},
			Resolution: "Check that " + cfgPath + " is valid JSON and readable.",
		}
	}
	if len(cfg) == 0 {
		return Finding{OK: true, Title: "Global runtimes config is empty"}
	}

	type problem struct {
		runtimeName string
		version     string
		displayName string
		detail      string
	}
	var problems []problem

	for name, version := range cfg {
		p, err := c.getProvider(name)
		if err != nil {
			problems = append(problems, problem{
				runtimeName: name,
				version:     version,
				displayName: name,
				detail:      "configured runtime is unknown to dtvem (no provider registered)",
			})
			continue
		}

		installed, err := p.IsInstalled(version)
		if err != nil {
			problems = append(problems, problem{
				runtimeName: name,
				version:     version,
				displayName: p.DisplayName(),
				detail:      fmt.Sprintf("could not check install status: %v", err),
			})
			continue
		}
		if !installed {
			problems = append(problems, problem{
				runtimeName: name,
				version:     version,
				displayName: p.DisplayName(),
				detail:      fmt.Sprintf("version %s is not installed (run `dtvem install %s %s`)", version, name, version),
			})
		}
	}

	if len(problems) == 0 {
		return Finding{OK: true, Title: "All configured runtimes are installed"}
	}

	// Stable order so the report doesn't shuffle between runs.
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].runtimeName == problems[j].runtimeName {
			return problems[i].version < problems[j].version
		}
		return problems[i].runtimeName < problems[j].runtimeName
	})

	details := make([]Detail, 0, len(problems)+1)
	details = append(details, Detail{Key: "Config", Value: cfgPath})
	for _, p := range problems {
		details = append(details, Detail{Key: p.displayName, Value: p.detail})
	}

	return Finding{
		Severity:   SeverityError,
		Title:      fmt.Sprintf("%d configured runtime version%s not installed", len(problems), plural(len(problems), "", "s")),
		Details:    details,
		Resolution: configuredRuntimesResolution(problems[0].runtimeName, problems[0].version),
	}
}

// configuredRuntimesResolution suggests the install command for the
// first problem so the user has a concrete next step. Listing every
// install command in the resolution would duplicate the detail block
// without adding info.
func configuredRuntimesResolution(runtimeName, version string) string {
	return strings.Join([]string{
		"Install the missing version(s) listed above, or edit",
		"  " + config.GlobalConfigPath(),
		"to reference versions that are installed.",
		"",
		fmt.Sprintf("Example: dtvem install %s %s", runtimeName, version),
	}, "\n")
}

func init() {
	Register(newConfiguredRuntimesCheck())
}
