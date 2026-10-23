package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// testSigner is a throwaway OpenPGP key for signing test data.
type testSigner struct {
	entity      *openpgp.Entity
	armoredKey  []byte
	fingerprint string
}

func newTestSigner(t *testing.T) *testSigner {
	t.Helper()
	entity, err := openpgp.NewEntity("dtvem test", "", "test@example.invalid", &packet.Config{Algorithm: packet.PubKeyAlgoEdDSA})
	if err != nil {
		t.Fatalf("NewEntity: %v", err)
	}
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	if err != nil {
		t.Fatalf("armor.Encode: %v", err)
	}
	if err := entity.Serialize(w); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing armor: %v", err)
	}
	return &testSigner{
		entity:      entity,
		armoredKey:  buf.Bytes(),
		fingerprint: strings.ToUpper(hex.EncodeToString(entity.PrimaryKey.Fingerprint)),
	}
}

func (s *testSigner) sign(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := openpgp.ArmoredDetachSign(&buf, s.entity, bytes.NewReader(data), nil); err != nil {
		t.Fatalf("ArmoredDetachSign: %v", err)
	}
	return buf.Bytes()
}

func (s *testSigner) verifier(t *testing.T) *signatureVerifier {
	t.Helper()
	v, err := newSignatureVerifier(s.armoredKey, s.fingerprint)
	if err != nil {
		t.Fatalf("newSignatureVerifier: %v", err)
	}
	return v
}

func TestPythonWindowsSigningKey(t *testing.T) {
	if _, err := newPythonWindowsVerifier(); err != nil {
		t.Fatalf("embedded signing key: %v", err)
	}

	// A checkout with CRLF line endings must load the same key.
	crlf := bytes.ReplaceAll(pythonWindowsSigningKey, []byte("\n"), []byte("\r\n"))
	if _, err := newSignatureVerifier(crlf, pythonWindowsSigningFingerprint); err != nil {
		t.Errorf("embedded signing key with CRLF line endings: %v", err)
	}
}

func TestNewSignatureVerifier_WrongFingerprint(t *testing.T) {
	signer := newTestSigner(t)
	if _, err := newSignatureVerifier(signer.armoredKey, pythonWindowsSigningFingerprint); err == nil {
		t.Error("newSignatureVerifier() accepted a key with a different fingerprint")
	}
}

func TestSignatureVerifier(t *testing.T) {
	signer := newTestSigner(t)
	other := newTestSigner(t)
	v := signer.verifier(t)
	data := []byte("python msi contents")

	if err := v.verify(data, signer.sign(t, data)); err != nil {
		t.Errorf("verify(good signature) = %v, want nil", err)
	}
	if err := v.verify([]byte("tampered contents"), signer.sign(t, data)); err == nil {
		t.Error("verify(tampered data) = nil, want error")
	}
	if err := v.verify(data, other.sign(t, data)); err == nil {
		t.Error("verify(signature by another key) = nil, want error")
	}
	if err := v.verify(data, []byte("not a signature")); err == nil {
		t.Error("verify(garbage) = nil, want error")
	}
}

func TestParsePythonFTPVersions(t *testing.T) {
	index := `<a href="../">../</a>
<a href="2.7.18/">2.7.18/</a>
<a href="3.9.11/">3.9.11/</a>
<a href="3.13.0/">3.13.0/</a>
<a href="3.14/">3.14/</a>
<a href="3.13.0a1/">3.13.0a1/</a>
<a href="README.html">README.html</a>`

	got := parsePythonFTPVersions(index)
	want := []string{"2.7.18", "3.9.11", "3.13.0"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parsePythonFTPVersions() = %v, want %v", got, want)
	}
}

// msiListing returns a python.org amd64 folder listing with the given files.
func msiListing(files ...string) string {
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "<a href=%q>%s</a>\n", f, f)
	}
	return b.String()
}

func signedMSIFiles() []string {
	var files []string
	for _, c := range pythonMSIComponents {
		files = append(files, c+".msi", c+".msi.asc")
	}
	return append(files, "doc.msi", "doc.msi.asc", "pip.msi")
}

func TestHasSignedMSIs(t *testing.T) {
	if !hasSignedMSIs(msiListing(signedMSIFiles()...)) {
		t.Error("hasSignedMSIs(all components signed) = false, want true")
	}

	var unsigned []string
	for _, c := range pythonMSIComponents {
		unsigned = append(unsigned, c+".msi")
	}
	if hasSignedMSIs(msiListing(unsigned...)) {
		t.Error("hasSignedMSIs(no signatures) = true, want false")
	}

	if hasSignedMSIs(msiListing("core.msi", "core.msi.asc", "exe.msi", "exe.msi.asc")) {
		t.Error("hasSignedMSIs(missing components) = true, want false")
	}
}

func TestPythonMSISource_FetchVersions(t *testing.T) {
	var unsigned []string
	for _, c := range pythonMSIComponents {
		unsigned = append(unsigned, c+".msi")
	}
	pages := map[string]string{
		"/": msiListing("3.4.10/", "3.5.2/", "3.9.11/", "3.9.15/", "3.13.1/", "3.14.0/"),
		// 3.5.2's MSIs have no signatures.
		"/3.5.2/amd64/":  msiListing(unsigned...),
		"/3.9.11/amd64/": msiListing(signedMSIFiles()...),
		"/3.13.1/amd64/": msiListing(signedMSIFiles()...),
		// 3.9.15 is a source-only security release: no amd64 folder.
		// 3.4.10 predates the MSI installer and 3.14.0 is Sigstore-only;
		// neither should be requested.
		"/3.14.0/amd64/": msiListing(signedMSIFiles()...),
	}
	var requested pathRecorder
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested.record(r.URL.Path)
		page, ok := pages[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, page)
	}))
	defer srv.Close()

	src := &PythonMSISource{baseURL: srv.URL + "/"}
	jobs, err := src.FetchVersions()
	if err != nil {
		t.Fatalf("FetchVersions() error: %v", err)
	}

	var versions []string
	for _, j := range jobs {
		versions = append(versions, j.Version)
		if j.Platform != "windows-amd64" || j.Build == nil {
			t.Errorf("job %s: platform %q, Build set %v; want windows-amd64 with a Build step", j.Version, j.Platform, j.Build != nil)
		}
		if want := fmt.Sprintf("python/%s/windows-amd64.tar.gz", j.Version); j.R2Key != want {
			t.Errorf("job %s: R2Key %q, want %q", j.Version, j.R2Key, want)
		}
		if want := fmt.Sprintf("python/%s/windows-amd64.meta.json", j.Version); j.MetaKey != want {
			t.Errorf("job %s: MetaKey %q, want %q", j.Version, j.MetaKey, want)
		}
		if want := srv.URL + "/" + j.Version + "/amd64/"; j.URL != want {
			t.Errorf("job %s: URL %q, want %q", j.Version, j.URL, want)
		}
	}
	sort.Strings(versions)
	if want := []string{"3.13.1", "3.9.11"}; !reflect.DeepEqual(versions, want) {
		t.Errorf("FetchVersions() versions = %v, want %v", versions, want)
	}
	for _, p := range requested.list() {
		if strings.HasPrefix(p, "/3.4.10/") || strings.HasPrefix(p, "/3.14.0/") {
			t.Errorf("requested %s, outside the MSI-signed range", p)
		}
	}
}

// pathRecorder records requested paths; test servers handle requests on
// several goroutines at once.
type pathRecorder struct {
	mu    sync.Mutex
	paths []string
}

func (r *pathRecorder) record(p string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.paths = append(r.paths, p)
}

func (r *pathRecorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.paths...)
}

// fakeMSIContents is what the fake extractor writes for each component,
// mirroring the real MSIs' contents.
var fakeMSIContents = map[string][]string{
	"core":  {"python39.dll", "python3.dll"},
	"exe":   {"python.exe", "pythonw.exe", "vcruntime140.dll"},
	"lib":   {"Lib/os.py", "Lib/venv/__init__.py", "Lib/venv/scripts/nt/python.exe", "Lib/ensurepip/__init__.py", "Lib/ensurepip/_bundled/pip-22.0.4-py3-none-any.whl", "DLLs/_ssl.pyd"},
	"dev":   {"include/Python.h", "libs/python39.lib"},
	"tcltk": {"DLLs/_tkinter.pyd", "DLLs/tcl86t.dll", "Lib/tkinter/__init__.py", "tcl/tcl8.6/init.tcl"},
}

// fakeExtract stands in for msiextract: the MSI's content names the
// component, and the component's files are written into destDir.
func fakeExtract(msiPath, destDir string) error {
	component, err := os.ReadFile(msiPath)
	if err != nil {
		return err
	}
	for _, rel := range fakeMSIContents[strings.TrimPrefix(string(component), "msi:")] {
		p := filepath.Join(destDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte("contents of "+rel), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// msiServer serves each component's MSI, whose content is "msi:<name>",
// and its signature by signer.
func msiServer(t *testing.T, signer *testSigner) *httptest.Server {
	t.Helper()
	files := make(map[string][]byte)
	for _, c := range pythonMSIComponents {
		msi := []byte("msi:" + c)
		files["/"+c+".msi"] = msi
		files["/"+c+".msi.asc"] = signer.sign(t, msi)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// tarEntries lists the entries of a .tar.gz, directories ending in "/".
func tarEntries(t *testing.T, archive []byte) map[string]string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	tr := tar.NewReader(gz)
	entries := make(map[string]string)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("reading tar: %v", err)
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatalf("reading %s: %v", h.Name, err)
		}
		if !h.ModTime.Equal(archiveModTime) {
			t.Errorf("%s: ModTime %v, want %v", h.Name, h.ModTime, archiveModTime)
		}
		entries[h.Name] = string(data)
	}
	return entries
}

func TestBuildFromMSIs(t *testing.T) {
	signer := newTestSigner(t)
	srv := msiServer(t, signer)

	archive, err := buildFromMSIs(srv.URL+"/", signer.verifier(t), fakeExtract)
	if err != nil {
		t.Fatalf("buildFromMSIs() error: %v", err)
	}

	entries := tarEntries(t, archive)
	for _, files := range fakeMSIContents {
		for _, rel := range files {
			if got, ok := entries["python/"+rel]; !ok || got != "contents of "+rel {
				t.Errorf("archive entry python/%s = %q (present %v)", rel, got, ok)
			}
		}
	}
	for _, dir := range []string{"python/", "python/Lib/", "python/Lib/venv/scripts/nt/", "python/tcl/tcl8.6/"} {
		if _, ok := entries[dir]; !ok {
			t.Errorf("archive has no directory entry %s", dir)
		}
	}
	for name := range entries {
		if !strings.HasPrefix(name, "python/") {
			t.Errorf("archive entry %s is outside python/", name)
		}
	}

	again, err := buildFromMSIs(srv.URL+"/", signer.verifier(t), fakeExtract)
	if err != nil {
		t.Fatalf("second buildFromMSIs() error: %v", err)
	}
	if !bytes.Equal(archive, again) {
		t.Error("building the same release twice produced different archives")
	}
}

func TestBuildFromMSIs_BadSignature(t *testing.T) {
	srv := msiServer(t, newTestSigner(t))
	trusted := newTestSigner(t)

	if _, err := buildFromMSIs(srv.URL+"/", trusted.verifier(t), fakeExtract); err == nil {
		t.Error("buildFromMSIs() accepted MSIs signed by an untrusted key")
	}
}

func TestBuildFromMSIs_ExtractError(t *testing.T) {
	signer := newTestSigner(t)
	srv := msiServer(t, signer)
	failing := func(string, string) error { return fmt.Errorf("msiextract exploded") }

	if _, err := buildFromMSIs(srv.URL+"/", signer.verifier(t), failing); err == nil || !strings.Contains(err.Error(), "msiextract exploded") {
		t.Errorf("buildFromMSIs() error = %v, want the extract error", err)
	}
}

func TestValidateWindowsPython(t *testing.T) {
	full := []string{"python.exe", "Lib/os.py", "Lib/venv/__init__.py", "Lib/ensurepip/__init__.py", "include/Python.h", "libs/python39.lib"}
	withTk := append(append([]string{}, full...), "DLLs/_tkinter.pyd", "Lib/tkinter/__init__.py")

	tests := []struct {
		name      string
		files     []string
		requireTk bool
		wantErr   string
	}{
		{name: "full install", files: full},
		{name: "full install with tkinter", files: withTk, requireTk: true},
		{name: "tkinter required but missing", files: full, requireTk: true, wantErr: "DLLs/_tkinter.pyd"},
		{name: "embeddable ._pth", files: append(append([]string{}, full...), "python39._pth"), wantErr: "isolated mode"},
		{name: "no venv", files: []string{"python.exe", "Lib/os.py", "Lib/ensurepip/__init__.py", "include/Python.h", "libs/python39.lib"}, wantErr: "Lib/venv/__init__.py"},
		{name: "no import library", files: full[:5], wantErr: "libs/pythonXY.lib"},
		{name: "embeddable layout", files: []string{"python.exe", "python39.zip", "python39.dll"}, wantErr: "missing"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWindowsPython(tt.files, tt.requireTk)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("validateWindowsPython() = %v, want nil", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("validateWindowsPython() = %v, want error mentioning %q", err, tt.wantErr)
			}
		})
	}
}

// nupkg builds a NuGet package with the given entries.
func nupkg(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip Create %s: %v", name, err)
		}
		if _, err := io.WriteString(w, entries[name]); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip Close: %v", err)
	}
	return buf.Bytes()
}

func fullNuGetEntries() map[string]string {
	return map[string]string{
		"[Content_Types].xml":             "types",
		"_rels/.rels":                     "rels",
		"python.nuspec":                   "nuspec",
		"build/native/python.props":       "props",
		"package/services/metadata/x":     "meta",
		".signature.p7s":                  "sig",
		"tools/":                          "",
		"tools/python.exe":                "exe",
		"tools/Lib/os.py":                 "os",
		"tools/Lib/venv/__init__.py":      "venv",
		"tools/Lib/ensurepip/__init__.py": "ensurepip",
		"tools/include/Python.h":          "header",
		"tools/libs/python39.lib":         "lib",
		"tools/Lib/site-packages/setuptools/script%20(dev).tmpl": "template",
	}
}

func TestArchiveNuGetTools(t *testing.T) {
	archive, err := archiveNuGetTools(nupkg(t, fullNuGetEntries()))
	if err != nil {
		t.Fatalf("archiveNuGetTools() error: %v", err)
	}
	entries := tarEntries(t, archive)

	for rel, want := range map[string]string{
		"python.exe":       "exe",
		"Lib/os.py":        "os",
		"include/Python.h": "header",
		"Lib/site-packages/setuptools/script (dev).tmpl": "template",
	} {
		if got, ok := entries["python/"+rel]; !ok || got != want {
			t.Errorf("archive entry python/%s = %q (present %v), want %q", rel, got, ok, want)
		}
	}
	for name := range entries {
		if !strings.HasPrefix(name, "python/") {
			t.Errorf("archive entry %s is outside python/", name)
		}
		for _, dropped := range []string{"nuspec", "props", ".rels", "Content_Types", "p7s", "services"} {
			if strings.Contains(name, dropped) {
				t.Errorf("archive kept NuGet packaging file %s", name)
			}
		}
	}
}

func TestArchiveNuGetTools_Rejects(t *testing.T) {
	embeddable := fullNuGetEntries()
	embeddable["tools/python39._pth"] = "python39.zip"

	incomplete := fullNuGetEntries()
	delete(incomplete, "tools/Lib/venv/__init__.py")

	unsafe := fullNuGetEntries()
	unsafe["tools/../escape.txt"] = "x"

	for name, entries := range map[string]map[string]string{"embeddable": embeddable, "incomplete": incomplete, "unsafe path": unsafe} {
		if _, err := archiveNuGetTools(nupkg(t, entries)); err == nil {
			t.Errorf("archiveNuGetTools(%s) = nil error, want rejection", name)
		}
	}
	if _, err := archiveNuGetTools([]byte("not a zip")); err == nil {
		t.Error("archiveNuGetTools(not a zip) = nil error, want error")
	}
}

func TestVerifySHA512(t *testing.T) {
	data := []byte("package")
	sum := sha512.Sum512(data)
	good := base64.StdEncoding.EncodeToString(sum[:])

	if err := verifySHA512(data, good); err != nil {
		t.Errorf("verifySHA512(matching) = %v, want nil", err)
	}
	if err := verifySHA512([]byte("tampered"), good); err == nil {
		t.Error("verifySHA512(mismatch) = nil, want error")
	}
}

// nugetServer serves python.org-style NuGet packages: flat container
// indexes and packages, registration leaves and catalog entries.
func nugetServer(t *testing.T, indexes map[string][]string, packages map[string][]byte, hashes map[string]string) (*httptest.Server, *pathRecorder) {
	t.Helper()
	requested := &pathRecorder{}
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		requested.record(p)
		switch {
		case strings.HasPrefix(p, "/flat/") && strings.HasSuffix(p, "/index.json"):
			id := strings.TrimSuffix(strings.TrimPrefix(p, "/flat/"), "/index.json")
			versions, ok := indexes[id]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string][]string{"versions": versions})
		case strings.HasPrefix(p, "/flat/") && strings.HasSuffix(p, ".nupkg"):
			pkg, ok := packages[filepath.Base(p)]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = w.Write(pkg)
		case strings.HasPrefix(p, "/registration/"):
			key := strings.TrimSuffix(strings.TrimPrefix(p, "/registration/"), ".json")
			_, _ = fmt.Fprintf(w, `{"catalogEntry": "%s/catalog/%s.json"}`, srv.URL, strings.ReplaceAll(key, "/", "."))
		case strings.HasPrefix(p, "/catalog/"):
			key := strings.TrimSuffix(strings.TrimPrefix(p, "/catalog/"), ".json")
			hash, ok := hashes[key]
			if !ok {
				http.NotFound(w, r)
				return
			}
			_, _ = fmt.Fprintf(w, `{"packageHashAlgorithm": "SHA512", "packageHash": "%s"}`, hash)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, requested
}

func TestPythonNuGetSource_FetchVersions(t *testing.T) {
	srv, requested := nugetServer(t, map[string][]string{
		// x64 packages are not reliably full installs and must not be used.
		"python":      {"3.5.2", "3.13.1"},
		"pythonarm64": {"3.4.4", "3.9.7", "3.13.0-b1", "3.11.4"},
	}, nil, nil)
	src := &PythonNuGetSource{flatContainerURL: srv.URL + "/flat/", registrationURL: srv.URL + "/registration/"}

	jobs, err := src.FetchVersions()
	if err != nil {
		t.Fatalf("FetchVersions() error: %v", err)
	}

	var got []string
	for _, j := range jobs {
		got = append(got, j.Version+"/"+j.Platform)
		if j.Build == nil {
			t.Errorf("job %s/%s has no Build step", j.Version, j.Platform)
		}
		if want := fmt.Sprintf("python/%s/%s.tar.gz", j.Version, j.Platform); j.R2Key != want {
			t.Errorf("job %s: R2Key %q, want %q", j.Version, j.R2Key, want)
		}
	}
	want := []string{"3.9.7/windows-arm64", "3.11.4/windows-arm64"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FetchVersions() = %v, want %v", got, want)
	}
	if want := srv.URL + "/flat/pythonarm64/3.11.4/pythonarm64.3.11.4.nupkg"; jobs[1].URL != want {
		t.Errorf("arm64 job URL = %q, want %q", jobs[1].URL, want)
	}
	for _, p := range requested.list() {
		if strings.HasPrefix(p, "/flat/python/") {
			t.Errorf("requested %s; x64 NuGet packages should not be listed", p)
		}
	}
}

func TestNotFullBuildErrors(t *testing.T) {
	// validateWindowsPython's rejections must reach the mirror loop as
	// errNotFullBuild, so those jobs are skipped rather than failed.
	if err := validateWindowsPython([]string{"python.exe", "python39.zip"}, false); !errors.Is(err, errNotFullBuild) {
		t.Errorf("validateWindowsPython(incomplete) = %v, want errNotFullBuild", err)
	}

	embeddable := fullNuGetEntries()
	embeddable["tools/python39._pth"] = "python39.zip"
	if _, err := archiveNuGetTools(nupkg(t, embeddable)); !errors.Is(err, errNotFullBuild) {
		t.Errorf("archiveNuGetTools(embeddable) = %v, want errNotFullBuild", err)
	}

	incomplete := MirrorJob{Build: func() ([]byte, error) {
		return nil, validateWindowsPython(nil, false)
	}}
	if _, _, _, err := fetchBinary(incomplete, &Stats{}); !errors.Is(err, errNotFullBuild) {
		t.Errorf("fetchBinary(incomplete build) = %v, want errNotFullBuild", err)
	}
}

func TestBuildFromNuGet(t *testing.T) {
	pkg := nupkg(t, fullNuGetEntries())
	sum := sha512.Sum512(pkg)
	srv, _ := nugetServer(t,
		map[string][]string{"python": {}, "pythonarm64": {"3.11.4"}},
		map[string][]byte{"pythonarm64.3.11.4.nupkg": pkg},
		map[string]string{"pythonarm64.3.11.4": base64.StdEncoding.EncodeToString(sum[:])},
	)
	src := &PythonNuGetSource{flatContainerURL: srv.URL + "/flat/", registrationURL: srv.URL + "/registration/"}
	jobs, err := src.FetchVersions()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("FetchVersions() = %d jobs, %v; want 1 job", len(jobs), err)
	}

	archive, err := jobs[0].Build()
	if err != nil {
		t.Fatalf("Build() error: %v", err)
	}
	if _, ok := tarEntries(t, archive)["python/python.exe"]; !ok {
		t.Error("archive has no python/python.exe")
	}
}

func TestBuildFromNuGet_HashMismatch(t *testing.T) {
	pkg := nupkg(t, fullNuGetEntries())
	other := sha512.Sum512([]byte("a different package"))
	srv, _ := nugetServer(t,
		map[string][]string{"python": {}, "pythonarm64": {"3.11.4"}},
		map[string][]byte{"pythonarm64.3.11.4.nupkg": pkg},
		map[string]string{"pythonarm64.3.11.4": base64.StdEncoding.EncodeToString(other[:])},
	)
	src := &PythonNuGetSource{flatContainerURL: srv.URL + "/flat/", registrationURL: srv.URL + "/registration/"}
	jobs, err := src.FetchVersions()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("FetchVersions() = %d jobs, %v; want 1 job", len(jobs), err)
	}

	if _, err := jobs[0].Build(); err == nil || !strings.Contains(err.Error(), "SHA-512 mismatch") {
		t.Errorf("Build() error = %v, want a SHA-512 mismatch", err)
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		in     string
		want   [3]int
		wantOK bool
	}{
		{"3.9.11", [3]int{3, 9, 11}, true},
		{"3.13.0-b1", [3]int{}, false},
		{"3.14", [3]int{}, false},
		{"v3.9.11", [3]int{}, false},
	}
	for _, tt := range tests {
		got, ok := parseVersion(tt.in)
		if ok != tt.wantOK || (ok && got != tt.want) {
			t.Errorf("parseVersion(%q) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.wantOK)
		}
	}

	if compareVersions([3]int{3, 9, 11}, [3]int{3, 10, 0}) >= 0 {
		t.Error("compareVersions(3.9.11, 3.10.0) should be negative")
	}
}
