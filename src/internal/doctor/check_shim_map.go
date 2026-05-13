package doctor

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"sort"
	"strings"

	"github.com/CodingWithCalvin/dtvem.cli/src/internal/config"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/constants"
	"github.com/CodingWithCalvin/dtvem.cli/src/internal/shim"
)

// shimMapCheck looks for drift between the shim binaries on disk and
// the shim-map cache (~/.dtvem/cache/shim-map.json). The cache tells
// the shim executable which runtime owns each shim name; if it's
// missing entries for shim files that exist (or has entries for shims
// that have been deleted) the user sees confusing routing failures.
//
// Drift typically appears after manual file moves, a partial install
// rollback, or an older dtvem version that didn't update the cache on
// shim creation. The fix is `dtvem reshim`, which rebuilds the cache
// from disk authoritatively.
type shimMapCheck struct {
	shimsDir    func() string
	loadShimMap func() (shim.ShimMap, error)
	newManager  func() (rehasher, error)
}

func newShimMapCheck() *shimMapCheck {
	return &shimMapCheck{
		shimsDir:    func() string { return config.DefaultPaths().Shims },
		loadShimMap: shim.LoadShimMap,
		newManager: func() (rehasher, error) {
			m, err := shim.NewManager()
			if err != nil {
				return nil, err
			}
			return m, nil
		},
	}
}

func (shimMapCheck) Name() string { return "shim-map-cache" }

func (c shimMapCheck) Run() Finding {
	diskShims, err := listShimNamesOnDisk(c.shimsDir())
	if err != nil {
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not list shim files on disk",
			Details:    []Detail{{Key: "Error", Value: err.Error()}},
			Resolution: "Check that " + c.shimsDir() + " is readable.",
		}
	}

	cache, cacheErr := c.loadShimMap()
	// A missing cache file is reported by the shim package as
	// os.IsNotExist; treat it as a present-but-empty cache so the
	// downstream comparison reports it as drift when shim files
	// exist on disk.
	if cacheErr != nil && !os.IsNotExist(cacheErr) {
		return Finding{
			Severity:   SeverityWarning,
			Title:      "Could not read shim-map cache",
			Details:    []Detail{{Key: "Error", Value: cacheErr.Error()}},
			Resolution: "Run `dtvem reshim` to rebuild the cache from disk.",
		}
	}
	if cache == nil {
		cache = shim.ShimMap{}
	}

	missingInCache, missingOnDisk := diffShimsAgainstCache(diskShims, cache)
	if len(missingInCache) == 0 && len(missingOnDisk) == 0 {
		// Handle the no-shims-installed case explicitly so the title
		// reads correctly: "matches cache" when there's nothing on
		// either side would be technically accurate but confusing.
		if len(diskShims) == 0 && len(cache) == 0 {
			return Finding{OK: true, Title: "No shims to check (cache and shims dir both empty)"}
		}
		return Finding{OK: true, Title: "Shim-map cache matches disk"}
	}

	details := make([]Detail, 0, 4)
	if len(missingInCache) > 0 {
		details = append(details, Detail{
			Key:   "On disk, missing from cache",
			Value: summarizeNames(missingInCache),
		})
	}
	if len(missingOnDisk) > 0 {
		details = append(details, Detail{
			Key:   "In cache, missing on disk",
			Value: summarizeNames(missingOnDisk),
		})
	}

	return Finding{
		Severity:   SeverityWarning,
		Title:      "Shim-map cache is out of sync with disk",
		Details:    details,
		Resolution: "Run `dtvem reshim` to rebuild the cache from the installed runtimes.",
		Fix: func() error {
			m, err := c.newManager()
			if err != nil {
				return fmt.Errorf("could not create shim manager: %w", err)
			}
			if _, err := m.Rehash(); err != nil {
				return fmt.Errorf("reshim failed: %w", err)
			}

			// Rehash rebuilds the cache from the installed runtimes
			// on disk, but it doesn't delete orphan shim files (shim
			// binaries in shims/ whose name isn't provided by any
			// installed runtime version's executable list). If we
			// still see drift afterward, surface that distinctly so
			// the user understands the next step is to delete the
			// stray shim file by hand rather than re-running fix.
			shim.ResetShimMapCache()
			diskShimsAfter, listErr := listShimNamesOnDisk(c.shimsDir())
			if listErr != nil {
				// We can't determine post-fix state; trust that
				// reshim succeeded and let the next doctor run
				// re-evaluate. Returning an error here would
				// misleadingly tell the user the fix failed.
				return nil //nolint:nilerr
			}
			cacheAfter, loadErr := c.loadShimMap()
			if loadErr != nil && !os.IsNotExist(loadErr) {
				return nil //nolint:nilerr
			}
			if cacheAfter == nil {
				cacheAfter = shim.ShimMap{}
			}
			missingInCache, missingOnDisk := diffShimsAgainstCache(diskShimsAfter, cacheAfter)
			if len(missingInCache) == 0 && len(missingOnDisk) == 0 {
				return nil
			}

			// Construct a focused message that names the still-broken
			// shims and the manual step needed — typically `rm` on
			// the orphan file, since reshim won't delete shims whose
			// underlying runtime executable isn't installed anywhere.
			var parts []string
			if len(missingInCache) > 0 {
				parts = append(parts, fmt.Sprintf("orphan shim file(s) %s — delete them from %s manually",
					summarizeNames(missingInCache), c.shimsDir()))
			}
			if len(missingOnDisk) > 0 {
				parts = append(parts, fmt.Sprintf("stale cache entrie(s) %s — re-run `dtvem reshim` after fixing the disk state",
					summarizeNames(missingOnDisk)))
			}
			return fmt.Errorf("reshim ran but drift remains: %s", strings.Join(parts, "; "))
		},
	}
}

// listShimNamesOnDisk returns the set of shim names present in
// shimsDir, normalized to bare names (no .exe / .cmd suffix on
// Windows) so they can be compared apples-to-apples with shim-map
// cache keys. A missing directory is not an error here: doctor calls
// this from a non-fatal check and "no shims dir" simply means an
// empty result, which the caller can interpret.
func listShimNamesOnDisk(shimsDir string) ([]string, error) {
	entries, err := os.ReadDir(shimsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	seen := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if goruntime.GOOS == constants.OSWindows {
			ext := filepath.Ext(name)
			// Skip .cmd/.bat wrappers so we don't double-count a shim
			// that has both a .exe and a .cmd companion.
			if ext == constants.ExtCmd || ext == constants.ExtBat {
				continue
			}
			name = strings.TrimSuffix(name, ext)
		}
		seen[name] = struct{}{}
	}

	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

// diffShimsAgainstCache returns the names present on disk but absent
// from the cache, and the names present in the cache but absent from
// disk. Both slices are sorted so the report output is stable across
// runs (map iteration in Go is intentionally randomized).
func diffShimsAgainstCache(diskShims []string, cache shim.ShimMap) (missingInCache, missingOnDisk []string) {
	cacheSet := make(map[string]struct{}, len(cache))
	for name := range cache {
		cacheSet[name] = struct{}{}
	}
	diskSet := make(map[string]struct{}, len(diskShims))
	for _, name := range diskShims {
		diskSet[name] = struct{}{}
		if _, ok := cacheSet[name]; !ok {
			missingInCache = append(missingInCache, name)
		}
	}
	for name := range cache {
		if _, ok := diskSet[name]; !ok {
			missingOnDisk = append(missingOnDisk, name)
		}
	}
	sort.Strings(missingInCache)
	sort.Strings(missingOnDisk)
	return missingInCache, missingOnDisk
}

// summarizeNames joins names with commas and truncates long lists so
// the rendered Detail value fits on one terminal line. We keep the
// first few items and indicate how many more were elided rather than
// trying to fit everything.
func summarizeNames(names []string) string {
	const maxShown = 5
	if len(names) <= maxShown {
		return strings.Join(names, ", ")
	}
	return fmt.Sprintf("%s, ... (+%d more)", strings.Join(names[:maxShown], ", "), len(names)-maxShown)
}

func init() {
	Register(newShimMapCheck())
}
