//go:build !windows

package doctor

import "errors"

// errNotWindows is returned by the Unix stub of
// removeStaleShimsFromUserPathImpl. It exists as a named sentinel so
// callers can identify the "called on the wrong platform" branch
// without asserting on error text.
var errNotWindows = errors.New("registry-based PATH fix is Windows-only")

// removeStaleShimsFromUserPathImpl is unreachable from the stale-shims
// check's Unix branch — the check only routes here on Windows — but
// Go still needs a definition for the symbol to compile. We return a
// distinguishable sentinel error so any future code path that
// accidentally calls it on Unix surfaces the mistake instead of
// silently returning success.
func removeStaleShimsFromUserPathImpl(string) ([]string, error) {
	return nil, errNotWindows
}
