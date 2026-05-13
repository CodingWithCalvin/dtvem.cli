//go:build windows

package doctor

import "github.com/CodingWithCalvin/dtvem.cli/src/internal/path"

// removeStaleShimsFromUserPathImpl forwards to the Windows-only
// registry helper. Wrapping it through this file lets the check's
// default-wiring constructor stay platform-agnostic — the staleShims
// check imports nothing from the registry directly.
func removeStaleShimsFromUserPathImpl(currentShimsDir string) ([]string, error) {
	return path.RemoveStaleShimsFromUserPath(currentShimsDir)
}
