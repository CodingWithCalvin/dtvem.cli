package doctor

import (
	"errors"
	"testing"
)

// fakeCheck is a Check implementation backed by closures, used to drive
// the registry and runner tests without standing up real environment
// state.
type fakeCheck struct {
	name string
	run  func() Finding
}

func (f *fakeCheck) Name() string { return f.name }
func (f *fakeCheck) Run() Finding { return f.run() }

func okFinding(title string) Finding {
	return Finding{OK: true, Title: title}
}

func errFinding(title string, fix func() error) Finding {
	return Finding{
		Severity:   SeverityError,
		Title:      title,
		Resolution: "fix it",
		Fix:        fix,
	}
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		sev  Severity
		want string
	}{
		{SeverityInfo, "info"},
		{SeverityWarning, "warning"},
		{SeverityError, "error"},
		{Severity(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.sev.String(); got != tc.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tc.sev, got, tc.want)
		}
	}
}

func TestFinding_Fixable(t *testing.T) {
	manual := Finding{}
	if manual.Fixable() {
		t.Error("Finding with nil Fix should not be Fixable()")
	}

	auto := Finding{Fix: func() error { return nil }}
	if !auto.Fixable() {
		t.Error("Finding with non-nil Fix should be Fixable()")
	}
}

func TestRegistry_RegistersAndReturnsCopy(t *testing.T) {
	r := NewRegistry()
	c1 := &fakeCheck{name: "a"}
	c2 := &fakeCheck{name: "b"}
	r.Register(c1)
	r.Register(c2)

	all := r.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d checks, want 2", len(all))
	}
	if all[0].Name() != "a" || all[1].Name() != "b" {
		t.Errorf("All() preserved order: got [%s, %s], want [a, b]", all[0].Name(), all[1].Name())
	}

	// Mutating the returned slice must not affect the registry's
	// internal state.
	all[0] = &fakeCheck{name: "mutated"}
	again := r.All()
	if again[0].Name() != "a" {
		t.Errorf("registry leaked internal slice: post-mutation got %q, want %q", again[0].Name(), "a")
	}
}

func TestRegistry_Reset(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeCheck{name: "a"})
	r.Reset()
	if got := r.All(); len(got) != 0 {
		t.Errorf("Reset() did not clear registry: got %d checks", len(got))
	}
}

func TestRun_PreservesOrderAndPairing(t *testing.T) {
	a := &fakeCheck{name: "a", run: func() Finding { return okFinding("a-title") }}
	b := &fakeCheck{name: "b", run: func() Finding { return errFinding("b-title", nil) }}

	res := Run([]Check{a, b})
	if len(res.Results) != 2 {
		t.Fatalf("Run returned %d results, want 2", len(res.Results))
	}
	if res.Results[0].Check.Name() != "a" || res.Results[0].Finding.Title != "a-title" {
		t.Errorf("result 0 mispaired: %#v", res.Results[0])
	}
	if res.Results[1].Check.Name() != "b" || res.Results[1].Finding.Title != "b-title" {
		t.Errorf("result 1 mispaired: %#v", res.Results[1])
	}
}

func TestResult_HasErrors(t *testing.T) {
	tests := []struct {
		name string
		res  Result
		want bool
	}{
		{
			name: "no findings",
			res:  Result{},
			want: false,
		},
		{
			name: "all OK",
			res: Result{Results: []CheckResult{
				{Finding: okFinding("a")},
				{Finding: okFinding("b")},
			}},
			want: false,
		},
		{
			name: "warning only",
			res: Result{Results: []CheckResult{
				{Finding: Finding{Severity: SeverityWarning, Title: "w"}},
			}},
			want: false,
		},
		{
			name: "OK error is ignored (OK=true)",
			res: Result{Results: []CheckResult{
				// A passing check should never cause HasErrors to fire,
				// even if Severity is somehow Error.
				{Finding: Finding{OK: true, Severity: SeverityError, Title: "ok-but-error-sev"}},
			}},
			want: false,
		},
		{
			name: "actual error",
			res: Result{Results: []CheckResult{
				{Finding: errFinding("real error", nil)},
			}},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.res.HasErrors(); got != tc.want {
				t.Errorf("HasErrors() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestResult_Fixable(t *testing.T) {
	fixerCalled := false
	fix := func() error { fixerCalled = true; return nil }

	res := Result{Results: []CheckResult{
		{Check: &fakeCheck{name: "ok"}, Finding: okFinding("passing")},
		{Check: &fakeCheck{name: "manual"}, Finding: errFinding("manual", nil)},
		{Check: &fakeCheck{name: "auto"}, Finding: errFinding("auto", fix)},
	}}

	fixable := res.Fixable()
	if len(fixable) != 1 {
		t.Fatalf("Fixable() returned %d, want 1 (only non-OK with Fix)", len(fixable))
	}
	if fixable[0].Check.Name() != "auto" {
		t.Errorf("Fixable() returned wrong check: %q, want %q", fixable[0].Check.Name(), "auto")
	}

	// The Fix closure should not have been invoked just by listing —
	// remediation is opt-in via the caller, not the runner.
	if fixerCalled {
		t.Error("Fixable() must not invoke Fix")
	}

	// Verify the returned closure is still callable.
	if err := fixable[0].Finding.Fix(); err != nil {
		t.Errorf("Fix() returned error: %v", err)
	}
	if !fixerCalled {
		t.Error("Fix() did not execute the underlying closure")
	}
}

func TestResult_Fixable_PropagatesErrors(t *testing.T) {
	// Sanity check that the Finding.Fix surface returns errors verbatim —
	// the doctor command relies on this to report fix failures to users.
	want := errors.New("could not write registry")
	res := Result{Results: []CheckResult{
		{Finding: errFinding("bad", func() error { return want })},
	}}
	got := res.Fixable()[0].Finding.Fix()
	if !errors.Is(got, want) {
		t.Errorf("Fix() returned %v, want %v", got, want)
	}
}

func TestPackageLevelRegistry_RegisterAndReset(t *testing.T) {
	// Snapshot existing registrations so we don't disturb checks
	// registered by other packages' init() functions.
	saved := All()
	Reset()
	defer func() {
		Reset()
		for _, c := range saved {
			Register(c)
		}
	}()

	Register(&fakeCheck{name: "x"})
	got := All()
	if len(got) != 1 || got[0].Name() != "x" {
		t.Errorf("package-level Register/All round-trip failed: got %#v", got)
	}
}
