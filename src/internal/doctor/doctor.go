// Package doctor diagnoses dtvem configuration issues and, when asked,
// applies fixes for the subset of findings that can be safely remediated
// in-place.
//
// The package is organized around a small Check interface. Each check
// inspects one aspect of the environment (PATH, shims directory, config
// files) and returns a Finding describing what it saw. The doctor command
// in src/cmd/doctor.go wires those findings into a report and, with --fix,
// invokes the per-finding Fix closure where one was supplied.
//
// Checks register themselves into the package-level registry via init(),
// the same pattern used by runtime and migration providers, so adding a
// new check is a self-contained file: write the Check implementation and
// add a Register() call.
package doctor

// Severity is the priority of a Finding. Severities only carry meaning
// when Finding.OK is false — a passing check has no severity.
type Severity int

const (
	// SeverityInfo describes a non-problem worth surfacing (e.g.,
	// "you're on the User PATH install path, just FYI").
	SeverityInfo Severity = iota
	// SeverityWarning describes a misconfiguration that doesn't currently
	// break anything but is likely to bite the user (e.g., a system
	// runtime taking precedence over a dtvem shim).
	SeverityWarning
	// SeverityError describes a misconfiguration that is actively breaking
	// dtvem. The doctor command exits non-zero when any error-severity
	// finding is present.
	SeverityError
)

// String returns a stable lowercase identifier for the severity.
func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	default:
		return "unknown"
	}
}

// Detail is a single key/value pair attached to a Finding, used to
// surface the specific data behind the finding (e.g., "Found:" / the
// stale path the check actually saw). Multiple Details are rendered as
// an aligned column under the title.
type Detail struct {
	Key   string
	Value string
}

// Finding is the result of running a single check.
//
// A check that found nothing wrong returns OK=true; callers should treat
// OK findings as "all green" regardless of severity. The Resolution
// string is informational text shown to the user — either a one-line
// description of what Fix() will do (for fixable findings) or the
// manual remediation steps (for findings without a Fix).
type Finding struct {
	// Severity is meaningful only when OK is false.
	Severity Severity

	// OK is true when the check passed and no problem was detected.
	OK bool

	// Title is a short, human-readable description of the check or the
	// problem. Always populated.
	Title string

	// Details is an ordered list of key/value pairs to render under the
	// title, in the order added. Empty for passing checks.
	Details []Detail

	// Resolution describes the remediation. For fixable findings this
	// is a one-line summary of what Fix() will do; for manual findings
	// it is the step-by-step instructions for the user.
	Resolution string

	// Fix, when non-nil, applies an automatic remediation for this
	// finding. Checks that require manual intervention leave this nil.
	Fix func() error
}

// Fixable reports whether the finding can be remediated automatically.
func (f Finding) Fixable() bool {
	return f.Fix != nil
}

// Check is the interface every doctor diagnostic implements.
//
// Run() must be side-effect-free: it inspects state and returns a
// Finding. Any state mutation lives in Finding.Fix, which the doctor
// command invokes only when the user has opted in via --fix.
type Check interface {
	// Name returns a stable identifier for this check (e.g.
	// "stale-shims-path"). Used in verbose output and tests.
	Name() string

	// Run executes the check and returns a Finding describing what it
	// observed. Run must not modify any state.
	Run() Finding
}
