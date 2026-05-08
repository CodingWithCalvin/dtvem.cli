package shim

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
)

// ErrSecondaryExecutableNotFound indicates that a secondary executable
// (e.g., "uv" given the python runtime path) could not be located in the
// runtime's install tree. Callers should surface this as a user-visible
// error rather than silently falling back to the runtime binary.
var ErrSecondaryExecutableNotFound = fmt.Errorf("secondary executable not found")

// FindSecondaryExecutable searches a runtime's install tree for a named
// secondary executable (e.g., "pip" or "uv" for python, "npm" for node).
//
// runtimeExePath is the absolute path to the primary runtime executable
// (e.g., python.exe, node, ruby). The function searches sibling directories
// commonly used for runtime-installed scripts: the runtime's own directory,
// a Scripts/ subdirectory (Python on Windows), and a parent-level Scripts/
// directory (alternate Python layout).
//
// On Windows, .cmd is preferred over .exe because tools like npm install
// .cmd shims that wrap Node scripts.
//
// Returns the absolute path on success, or ErrSecondaryExecutableNotFound
// (wrapped with the requested name) if no candidate exists. Callers should
// not fall back to runtimeExePath — doing so silently runs the runtime
// binary as if it were the requested command.
func FindSecondaryExecutable(runtimeExePath, name string) (string, error) {
	dir := filepath.Dir(runtimeExePath)

	searchDirs := []string{
		dir,
		filepath.Join(dir, "Scripts"),
		filepath.Join(dir, "..", "Scripts"),
	}

	if runtime.GOOS == constants.OSWindows {
		for _, searchDir := range searchDirs {
			candidate := filepath.Join(searchDir, name)
			if _, err := os.Stat(candidate + constants.ExtCmd); err == nil {
				return candidate + constants.ExtCmd, nil
			}
			if _, err := os.Stat(candidate + constants.ExtExe); err == nil {
				return candidate + constants.ExtExe, nil
			}
		}
	} else {
		for _, searchDir := range searchDirs {
			candidate := filepath.Join(searchDir, name)
			if _, err := os.Stat(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	return "", fmt.Errorf("%w: %s", ErrSecondaryExecutableNotFound, name)
}
