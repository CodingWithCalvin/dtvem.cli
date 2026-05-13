package doctor

import (
	"strings"
	"testing"
)

func TestShimsInPathCheck_Pass(t *testing.T) {
	_, shims := withDtvemRoot(t)
	withPathEnv(t, []string{shims, "/usr/bin"})

	got := newShimsInPathCheck().Run()
	if !got.OK {
		t.Errorf("expected OK when shims dir is in PATH, got %#v", got)
	}
}

func TestShimsInPathCheck_FailWhenMissing(t *testing.T) {
	_, shims := withDtvemRoot(t)
	// Build a PATH that explicitly omits the shims dir.
	withPathEnv(t, []string{"/usr/bin", "/usr/local/bin"})

	got := newShimsInPathCheck().Run()
	if got.OK {
		t.Fatalf("expected non-OK finding when shims dir absent from PATH, got OK")
	}
	if got.Severity != SeverityError {
		t.Errorf("severity: got %s, want error", got.Severity)
	}
	if got.Fixable() {
		t.Errorf("shims-in-path check should be manual, but Finding.Fix is set")
	}

	// The expected shims path must be in details so the user can see
	// what's missing.
	foundExpected := false
	for _, d := range got.Details {
		if d.Key == "Expected" && d.Value == shims {
			foundExpected = true
			break
		}
	}
	if !foundExpected {
		t.Errorf("expected shims path %q in details, got %#v", shims, got.Details)
	}

	if !strings.Contains(got.Resolution, "dtvem init") {
		t.Errorf("resolution should reference `dtvem init`, got %q", got.Resolution)
	}
}

func TestShimsInPathCheck_Registered(t *testing.T) {
	found := false
	for _, c := range All() {
		if c.Name() == "shims-in-path" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("shims-in-path check is not in the default registry")
	}
}
