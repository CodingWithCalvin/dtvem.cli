package doctor

// CheckResult pairs a check with the finding it produced. The doctor
// command iterates a []CheckResult to render the report; tests use it
// to assert on the structured output rather than parsing rendered text.
type CheckResult struct {
	Check   Check
	Finding Finding
}

// Result is the aggregated outcome of running every registered check.
type Result struct {
	// Results is one entry per check, in the order the checks ran.
	Results []CheckResult
}

// HasErrors reports whether any non-OK finding has SeverityError. The
// doctor command uses this to decide its process exit code.
func (r Result) HasErrors() bool {
	for _, cr := range r.Results {
		if !cr.Finding.OK && cr.Finding.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Fixable returns the subset of results that have an automatic fix
// available and have not passed. Used by the doctor command to drive
// the per-finding remediation prompts.
func (r Result) Fixable() []CheckResult {
	var out []CheckResult
	for _, cr := range r.Results {
		if !cr.Finding.OK && cr.Finding.Fixable() {
			out = append(out, cr)
		}
	}
	return out
}

// Run executes every check in the given slice and returns the aggregated
// Result. Checks run sequentially in the order provided; we deliberately
// avoid parallelism because several checks read shared environment state
// (PATH, registry, files under ~/.dtvem) and a stable order keeps the
// rendered report easy to read.
func Run(checks []Check) Result {
	res := Result{Results: make([]CheckResult, 0, len(checks))}
	for _, c := range checks {
		res.Results = append(res.Results, CheckResult{
			Check:   c,
			Finding: c.Run(),
		})
	}
	return res
}

// RunAll is shorthand for Run(All()).
func RunAll() Result {
	return Run(All())
}
