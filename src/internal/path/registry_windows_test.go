//go:build windows

package path

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/sys/windows/registry"
)

// fakePathKey is an in-memory pathKey holding one Path value.
type fakePathKey struct {
	value   string
	valType uint32
	missing bool
	readErr error
	writes  int
}

func (k *fakePathKey) GetStringValue(name string) (string, uint32, error) {
	if k.readErr != nil {
		return "", 0, k.readErr
	}
	if k.missing {
		return "", 0, registry.ErrNotExist
	}
	return k.value, k.valType, nil
}

func (k *fakePathKey) SetStringValue(name, value string) error {
	k.value, k.valType, k.missing = value, registry.SZ, false
	k.writes++
	return nil
}

func (k *fakePathKey) SetExpandStringValue(name, value string) error {
	k.value, k.valType, k.missing = value, registry.EXPAND_SZ, false
	k.writes++
	return nil
}

func typeName(valType uint32) string {
	switch valType {
	case registry.SZ:
		return "REG_SZ"
	case registry.EXPAND_SZ:
		return "REG_EXPAND_SZ"
	default:
		return fmt.Sprintf("type %d", valType)
	}
}

func TestWritePathValue(t *testing.T) {
	tests := []struct {
		name     string
		original uint32
		value    string
		want     uint32
	}{
		{name: "REG_EXPAND_SZ stays expandable", original: registry.EXPAND_SZ, value: `C:\bin`, want: registry.EXPAND_SZ},
		{name: "REG_SZ without references stays REG_SZ", original: registry.SZ, value: `C:\bin;C:\tools`, want: registry.SZ},
		{name: "REG_SZ with a reference becomes expandable", original: registry.SZ, value: `C:\bin;%USERPROFILE%\.dotnet\tools`, want: registry.EXPAND_SZ},
		{name: "a lone percent sign is not a reference", original: registry.SZ, value: `C:\100%\bin`, want: registry.SZ},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := &fakePathKey{}
			if err := writePathValue(key, tt.value, tt.original); err != nil {
				t.Fatalf("writePathValue() error: %v", err)
			}
			if key.valType != tt.want || key.value != tt.value {
				t.Errorf("wrote %q as %s, want %q as %s", key.value, typeName(key.valType), tt.value, typeName(tt.want))
			}
		})
	}
}

func TestReadPathValue_Missing(t *testing.T) {
	value, valType, err := readPathValue(&fakePathKey{missing: true})
	if err != nil || value != "" || valType != registry.EXPAND_SZ {
		t.Errorf("readPathValue(missing) = (%q, %s, %v), want (\"\", REG_EXPAND_SZ, nil)", value, typeName(valType), err)
	}
}

func TestPrependShimsDir(t *testing.T) {
	const shims = `C:\Users\me\.dtvem\shims`
	key := &fakePathKey{
		value:   `%SystemRoot%\system32;C:\old\.dtvem\shims;C:\tools;` + shims + `;%USERPROFILE%\.dotnet\tools;`,
		valType: registry.EXPAND_SZ,
	}

	removed, err := prependShimsDir(key, "User", shims)
	if err != nil {
		t.Fatalf("prependShimsDir() error: %v", err)
	}

	want := shims + `;%SystemRoot%\system32;C:\tools;%USERPROFILE%\.dotnet\tools`
	if key.value != want {
		t.Errorf("PATH = %q, want %q", key.value, want)
	}
	if key.valType != registry.EXPAND_SZ {
		t.Errorf("PATH written as %s, want REG_EXPAND_SZ", typeName(key.valType))
	}
	if !reflect.DeepEqual(removed, []string{`C:\old\.dtvem\shims`}) {
		t.Errorf("removed = %v, want the stale shims entry", removed)
	}
}

func TestPrependShimsDir_KeepsType(t *testing.T) {
	const shims = `C:\Users\me\.dtvem\shims`
	tests := []struct {
		name  string
		key   *fakePathKey
		want  uint32
		value string
	}{
		{name: "REG_SZ without references", key: &fakePathKey{value: `C:\tools`, valType: registry.SZ}, want: registry.SZ, value: shims + `;C:\tools`},
		{name: "REG_SZ with a reference is repaired", key: &fakePathKey{value: `%SystemRoot%\system32`, valType: registry.SZ}, want: registry.EXPAND_SZ, value: shims + `;%SystemRoot%\system32`},
		{name: "no PATH yet", key: &fakePathKey{missing: true}, want: registry.EXPAND_SZ, value: shims},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := prependShimsDir(tt.key, "User", shims); err != nil {
				t.Fatalf("prependShimsDir() error: %v", err)
			}
			if tt.key.value != tt.value || tt.key.valType != tt.want {
				t.Errorf("wrote %q as %s, want %q as %s", tt.key.value, typeName(tt.key.valType), tt.value, typeName(tt.want))
			}
		})
	}
}

func TestPrependShimsDir_ReadError(t *testing.T) {
	key := &fakePathKey{readErr: errors.New("access denied")}

	_, err := prependShimsDir(key, "System", `C:\shims`)
	if err == nil || !strings.Contains(err.Error(), "System PATH") {
		t.Errorf("prependShimsDir() error = %v, want a System PATH read error", err)
	}
	if key.writes != 0 {
		t.Error("prependShimsDir() wrote PATH after a read error")
	}
}

func TestRemoveStaleShims(t *testing.T) {
	const current = `C:\Users\me\.dtvem\shims`
	key := &fakePathKey{
		value:   `C:\Users\me\.cargo\bin;C:\old\.dtvem\shims;` + current + `;%USERPROFILE%\.dotnet\tools`,
		valType: registry.EXPAND_SZ,
	}

	removed, err := removeStaleShims(key, current)
	if err != nil {
		t.Fatalf("removeStaleShims() error: %v", err)
	}

	want := `C:\Users\me\.cargo\bin;` + current + `;%USERPROFILE%\.dotnet\tools`
	if key.value != want || key.valType != registry.EXPAND_SZ {
		t.Errorf("wrote %q as %s, want %q as REG_EXPAND_SZ", key.value, typeName(key.valType), want)
	}
	if !reflect.DeepEqual(removed, []string{`C:\old\.dtvem\shims`}) {
		t.Errorf("removed = %v, want the stale shims entry", removed)
	}
}

func TestRemoveStaleShims_NothingToRemove(t *testing.T) {
	key := &fakePathKey{value: `C:\tools;%USERPROFILE%\bin`, valType: registry.SZ}

	removed, err := removeStaleShims(key, `C:\Users\me\.dtvem\shims`)
	if err != nil || len(removed) != 0 {
		t.Fatalf("removeStaleShims() = %v, %v; want nothing removed", removed, err)
	}
	if key.writes != 0 {
		t.Error("removeStaleShims() rewrote PATH with nothing to remove")
	}
}

func TestRepairPathType(t *testing.T) {
	const value = `C:\tools;%USERPROFILE%\.dotnet\tools`
	key := &fakePathKey{value: value, valType: registry.SZ}

	if err := repairPathType(key); err != nil {
		t.Fatalf("repairPathType() error: %v", err)
	}
	if key.value != value || key.valType != registry.EXPAND_SZ {
		t.Errorf("wrote %q as %s, want %q as REG_EXPAND_SZ", key.value, typeName(key.valType), value)
	}
}

// TestPrependShimsDir_Registry runs the helper against a real registry
// value: a scratch key under HKCU\Software, never the real environment.
func TestPrependShimsDir_Registry(t *testing.T) {
	scratch := fmt.Sprintf(`Software\dtvem-test-path-%d`, os.Getpid())
	key, _, err := registry.CreateKey(registry.CURRENT_USER, scratch, registry.ALL_ACCESS)
	if err != nil {
		t.Skipf("cannot create scratch registry key: %v", err)
	}
	t.Cleanup(func() {
		_ = key.Close()
		_ = registry.DeleteKey(registry.CURRENT_USER, scratch)
	})

	if err := key.SetExpandStringValue("Path", `%SystemRoot%\system32;C:\tools`); err != nil {
		t.Fatalf("seeding scratch Path: %v", err)
	}
	if _, err := prependShimsDir(key, "User", `C:\Users\me\.dtvem\shims`); err != nil {
		t.Fatalf("prependShimsDir() error: %v", err)
	}

	value, valType, err := key.GetStringValue("Path")
	if err != nil {
		t.Fatalf("reading scratch Path: %v", err)
	}
	if valType != registry.EXPAND_SZ {
		t.Errorf("scratch Path stored as %s, want REG_EXPAND_SZ", typeName(valType))
	}
	if want := `C:\Users\me\.dtvem\shims;%SystemRoot%\system32;C:\tools`; value != want {
		t.Errorf("scratch Path = %q, want %q", value, want)
	}
}
