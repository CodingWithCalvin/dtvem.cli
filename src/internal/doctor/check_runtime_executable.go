package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/runtime"
)

// runtimeExecutableCheck walks ~/.dtvem/versions and, for every
// installed runtime version, asks the provider where the executable
// should live and confirms it's actually there. Missing executables
// most commonly result from interrupted installs (the version
// directory was created but the download/extract didn't finish) or
// from manual edits to the versions tree.
//
// The fix isn't safe to automate. Reinstalling the runtime version
// is what the user wants, but doctor doing that without prompting
// could overwrite an incomplete install the user was intentionally
// pinning for forensics. We surface a one-line install command per
// missing version so remediation is one copy-paste away.
type runtimeExecutableCheck struct {
	versionsDir func() string
	getProvider func(name string) (runtime.ShimProvider, error)
}

func newRuntimeExecutableCheck() *runtimeExecutableCheck {
	return &runtimeExecutableCheck{
		versionsDir: func() string { return config.DefaultPaths().Versions },
		getProvider: runtime.GetShimProvider,
	}
}

func (runtimeExecutableCheck) Name() string { return "runtime-executable-present" }

func (c runtimeExecutableCheck) Run() Finding {
	root := c.versionsDir()
	installs, err := listInstalledVersions(root)
	if err != nil {
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not list installed runtime versions",
			Details:    []Detail{{Key: "Error", Value: err.Error()}},
			Resolution: "Check that " + root + " is readable.",
		}
	}
	if len(installs) == 0 {
		return Finding{OK: true, Title: "No installed runtime versions to check"}
	}

	type problem struct {
		displayName string
		runtimeName string
		version     string
		detail      string
	}
	var problems []problem

	for _, inst := range installs {
		p, err := c.getProvider(inst.runtimeName)
		if err != nil {
			problems = append(problems, problem{
				displayName: inst.runtimeName,
				runtimeName: inst.runtimeName,
				version:     inst.version,
				detail:      "no provider registered for this runtime — orphaned data?",
			})
			continue
		}

		execPath, err := p.ExecutablePath(inst.version)
		if err != nil {
			problems = append(problems, problem{
				displayName: p.DisplayName(),
				runtimeName: inst.runtimeName,
				version:     inst.version,
				detail:      fmt.Sprintf("provider could not resolve executable path: %v", err),
			})
			continue
		}
		if execPath == "" {
			problems = append(problems, problem{
				displayName: p.DisplayName(),
				runtimeName: inst.runtimeName,
				version:     inst.version,
				detail:      "provider returned an empty executable path",
			})
			continue
		}
		info, err := os.Stat(execPath)
		if err != nil {
			problems = append(problems, problem{
				displayName: p.DisplayName(),
				runtimeName: inst.runtimeName,
				version:     inst.version,
				detail:      fmt.Sprintf("expected at %s — %v", execPath, err),
			})
			continue
		}
		if info.IsDir() {
			problems = append(problems, problem{
				displayName: p.DisplayName(),
				runtimeName: inst.runtimeName,
				version:     inst.version,
				detail:      fmt.Sprintf("expected file but found directory at %s", execPath),
			})
		}
	}

	if len(problems) == 0 {
		return Finding{OK: true, Title: "All installed runtime versions have their executable"}
	}

	// Stable ordering so the report is deterministic.
	sort.Slice(problems, func(i, j int) bool {
		if problems[i].runtimeName == problems[j].runtimeName {
			return problems[i].version < problems[j].version
		}
		return problems[i].runtimeName < problems[j].runtimeName
	})

	details := make([]Detail, 0, len(problems))
	for _, p := range problems {
		details = append(details, Detail{
			Key:   fmt.Sprintf("%s %s", p.displayName, p.version),
			Value: p.detail,
		})
	}

	return Finding{
		Severity: SeverityError,
		Title:    fmt.Sprintf("%d installed runtime version%s missing its executable", len(problems), plural(len(problems), "", "s")),
		Details:  details,
		Resolution: strings.Join([]string{
			"Reinstall the affected version(s) to restore the executable. Example:",
			fmt.Sprintf("  dtvem uninstall %s %s && dtvem install %s %s",
				problems[0].runtimeName, problems[0].version,
				problems[0].runtimeName, problems[0].version),
		}, "\n"),
	}
}

// installedVersion is the on-disk (runtime, version) pair we encounter
// while walking ~/.dtvem/versions. Defined inline because it's only
// used by this check and exporting it would invite reuse elsewhere
// against an interface that isn't ours to stabilize yet.
type installedVersion struct {
	runtimeName string
	version     string
}

// listInstalledVersions returns every (runtime, version) pair found
// under versionsDir, sorted by runtime then version for stable output.
// A missing versionsDir returns (nil, nil) — that's the "no installs
// yet" state, not an error condition.
func listInstalledVersions(versionsDir string) ([]installedVersion, error) {
	runtimeEntries, err := os.ReadDir(versionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var out []installedVersion
	for _, r := range runtimeEntries {
		if !r.IsDir() {
			continue
		}
		versionEntries, err := os.ReadDir(filepath.Join(versionsDir, r.Name()))
		if err != nil {
			continue
		}
		for _, v := range versionEntries {
			if !v.IsDir() {
				continue
			}
			out = append(out, installedVersion{runtimeName: r.Name(), version: v.Name()})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].runtimeName == out[j].runtimeName {
			return out[i].version < out[j].version
		}
		return out[i].runtimeName < out[j].runtimeName
	})
	return out, nil
}

func init() {
	Register(newRuntimeExecutableCheck())
}
