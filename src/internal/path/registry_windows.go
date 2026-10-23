//go:build windows

package path

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"golang.org/x/sys/windows/registry"
)

const (
	userEnvironmentKey   = `Environment`
	systemEnvironmentKey = `SYSTEM\CurrentControlSet\Control\Session Manager\Environment`
)

// envReferencePattern matches a %VAR% reference in a PATH value.
var envReferencePattern = regexp.MustCompile(`%[^%;]+%`)

// pathKey is the part of registry.Key the PATH helpers use, so tests can
// use a fake instead of the real environment.
type pathKey interface {
	GetStringValue(name string) (val string, valtype uint32, err error)
	SetStringValue(name, value string) error
	SetExpandStringValue(name, value string) error
}

// readPathValue returns the PATH value under key, with %VAR% references
// unexpanded, and its registry type. A missing value reads as empty with
// REG_EXPAND_SZ, the type Windows gives PATH.
func readPathValue(key pathKey) (string, uint32, error) {
	value, valType, err := key.GetStringValue("Path")
	if errors.Is(err, registry.ErrNotExist) {
		return "", registry.EXPAND_SZ, nil
	}
	return value, valType, err
}

// writePathValue writes the PATH value under key, as REG_EXPAND_SZ when
// it was stored that way or when value holds a %VAR% reference. Windows
// only expands references in a REG_EXPAND_SZ value, so writing REG_SZ would
// leave entries like %SystemRoot%\system32 literal and off PATH.
func writePathValue(key pathKey, value string, valType uint32) error {
	if valType == registry.EXPAND_SZ || hasEnvReference(value) {
		return key.SetExpandStringValue("Path", value)
	}
	return key.SetStringValue("Path", value)
}

func hasEnvReference(value string) bool {
	return envReferencePattern.MatchString(value)
}

// expandPathEntry expands %VAR% references in a PATH entry, as Windows does
// for a REG_EXPAND_SZ value. An entry that can't be expanded is returned as
// is.
func expandPathEntry(entry string) string {
	expanded, err := registry.ExpandString(entry)
	if err != nil {
		return entry
	}
	return expanded
}

// prependShimsDir puts shimsDir first on the PATH under key, dropping any
// other spelling of it and any stale dtvem shims entries (see
// ClassifyShimsEntries). It returns the stale entries it removed, and
// other dtvem roots' existing shims entries, which it keeps. scope names
// the PATH ("User" or "System") in errors.
func prependShimsDir(key pathKey, scope, shimsDir string) (removedStale, otherRoots []string, err error) {
	current, valType, err := readPathValue(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read %s PATH: %w", scope, err)
	}

	classified := ClassifyShimsEntries(strings.Split(current, ";"), shimsDir)
	entries := []string{shimsDir}
	for _, e := range classified.Kept {
		if trimmed := strings.TrimSpace(e); !sameShimsDir(trimmed, shimsDir) {
			entries = append(entries, trimmed)
		}
	}

	if err := writePathValue(key, strings.Join(entries, ";"), valType); err != nil {
		return nil, nil, fmt.Errorf("failed to update %s PATH in registry: %w", scope, err)
	}
	return trimAll(classified.Stale), trimAll(classified.OtherRoots), nil
}

func trimAll(entries []string) []string {
	trimmed := make([]string, len(entries))
	for i, e := range entries {
		trimmed[i] = strings.TrimSpace(e)
	}
	return trimmed
}

// removeStaleShims removes stale dtvem shims entries from the PATH under
// key, keeping currentShimsDir and the order of everything else, and
// returns the entries it removed. PATH is only rewritten when something
// was removed.
func removeStaleShims(key pathKey, currentShimsDir string) ([]string, error) {
	current, valType, err := readPathValue(key)
	if err != nil {
		return nil, fmt.Errorf("failed to read User PATH: %w", err)
	}
	if current == "" {
		return nil, nil
	}

	kept, removed := PartitionStaleShimsEntries(strings.Split(current, ";"), currentShimsDir)
	if len(removed) == 0 {
		return nil, nil
	}
	if err := writePathValue(key, strings.Join(kept, ";"), valType); err != nil {
		return nil, fmt.Errorf("failed to update User PATH in registry: %w", err)
	}
	return removed, nil
}

// RegistryPaths reports how the User and System PATH values are stored.
func RegistryPaths() ([]RegistryPath, error) {
	scopes := []struct {
		name string
		root registry.Key
		path string
	}{
		{"User", registry.CURRENT_USER, userEnvironmentKey},
		{"System", registry.LOCAL_MACHINE, systemEnvironmentKey},
	}

	paths := make([]RegistryPath, 0, len(scopes))
	for _, s := range scopes {
		key, err := registry.OpenKey(s.root, s.path, registry.QUERY_VALUE)
		if err != nil {
			return nil, fmt.Errorf("failed to open %s environment: %w", s.name, err)
		}
		value, valType, err := readPathValue(key)
		_ = key.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read %s PATH: %w", s.name, err)
		}
		paths = append(paths, RegistryPath{
			Scope:         s.name,
			Expandable:    valType == registry.EXPAND_SZ,
			HasReferences: hasEnvReference(value),
		})
	}
	return paths, nil
}

// RepairUserPathType rewrites the User PATH as REG_EXPAND_SZ, with its
// entries unchanged, so its %VAR% references expand again.
func RepairUserPathType() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, userEnvironmentKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open User PATH registry key: %w", err)
	}
	defer func() { _ = key.Close() }()

	if err := repairPathType(key); err != nil {
		return err
	}
	broadcastSettingChange()
	return nil
}

// repairPathType rewrites the PATH under key as REG_EXPAND_SZ.
func repairPathType(key pathKey) error {
	value, _, err := readPathValue(key)
	if err != nil {
		return fmt.Errorf("failed to read User PATH: %w", err)
	}
	if err := key.SetExpandStringValue("Path", value); err != nil {
		return fmt.Errorf("failed to update User PATH in registry: %w", err)
	}
	return nil
}
