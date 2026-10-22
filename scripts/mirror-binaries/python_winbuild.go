package main

// Full Windows builds for Python versions python-build-standalone does not
// cover.
//
// python.org's embeddable zips, which the mirror used to fall back to, are a
// trimmed runtime meant to be shipped inside an application: a ._pth file
// puts them in isolated mode (no script directory on sys.path, PYTHONPATH
// ignored), and they have no venv, ensurepip, headers or tkinter. Instead the
// mirror assembles a full build of the same release from official files:
//
//   - windows-amd64: python.org's per-component MSIs, verified against the
//     Windows release signing key and unpacked with msiextract (msitools),
//     so this runs on a Linux runner;
//   - windows-arm64: python.org's NuGet packages, verified against
//     nuget.org's SHA-512.
//
// A release with neither (3.5.2's MSIs are unsigned) keeps its embeddable
// build, and dtvem's installer disables its ._pth file instead.
//
// Either way the result is a .tar.gz with everything under python/, the
// layout of python-build-standalone's Windows archives, so dtvem installs it
// without changes.

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha512"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

const (
	pythonFTPBaseURL     = "https://www.python.org/ftp/python/"
	nugetFlatContainer   = "https://api.nuget.org/v3-flatcontainer/"
	nugetRegistrationURL = "https://api.nuget.org/v3/registration5-gz-semver2/"

	// pythonWindowsSigningFingerprint is the key python.org lists for
	// "Windows binaries" (Steve Dower) at
	// https://www.python.org/downloads/metadata/pgp/.
	pythonWindowsSigningFingerprint = "7ED10B6531D7C8E1BC296021FC624643487034E5"

	// msiDiscoveryWorkers bounds concurrent requests to python.org while
	// checking which releases have signed MSIs.
	msiDiscoveryWorkers = 8
)

// pythonWindowsSigningKey is the armored public key for
// pythonWindowsSigningFingerprint, committed so the mirror never depends on
// a keyserver.
//
//go:embed keys/python-windows-signing.asc
var pythonWindowsSigningKey []byte

// pythonMSIComponents are the MSIs that together make a full install:
// pythonXY.dll, python.exe, the standard library (with venv and ensurepip),
// headers and import library, and tkinter with Tcl/Tk. pip.msi is left out:
// it holds no files and only runs ensurepip, which dtvem does itself.
var pythonMSIComponents = []string{"core", "exe", "lib", "dev", "tcltk"}

// pythonNuGetPackages maps a platform to python.org's NuGet package for it.
// Only arm64 is listed: python.org has no arm64 MSIs for 3.9 and 3.10, and
// every pythonarm64 release checked is a full install. x64 is covered by
// python-build-standalone and the MSIs, and its NuGet packages are not
// reliably full (3.5.2, 3.5.3 and 3.6.0 lack venv and ensurepip).
var pythonNuGetPackages = []struct {
	platform string
	id       string
}{
	{"windows-arm64", "pythonarm64"},
}

// errNotFullBuild marks an upstream build that turned out not to be a full
// install. The mirror skips such a job rather than failing: retrying cannot
// change upstream's files, and failing would fail every scheduled run.
var errNotFullBuild = errors.New("not a full Python install")

var (
	// minWindowsPython is the first release with python.org's MSI-based
	// installer and NuGet packages.
	minWindowsPython = [3]int{3, 5, 0}
	// firstSigstoreOnlyPython is the first release python.org signs with
	// Sigstore only; earlier MSIs carry OpenPGP signatures.
	firstSigstoreOnlyPython = [3]int{3, 14, 0}

	pythonFTPVersionPattern = regexp.MustCompile(`href="(\d+\.\d+\.\d+)/"`)
	releaseVersionPattern   = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

	// archiveModTime is the modification time of every archive entry, so
	// rebuilding the same release produces the same archive.
	archiveModTime = time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
)

// PythonMSISource assembles full windows-amd64 builds from python.org's
// per-component MSIs.
type PythonMSISource struct {
	// baseURL is python.org's FTP root; tests point it at a local server.
	baseURL string
	// extract unpacks one MSI into a directory; nil means msiextract.
	extract func(msiPath, destDir string) error
}

func (s *PythonMSISource) Name() string {
	return "python.org MSIs"
}

func (s *PythonMSISource) ftpBase() string {
	if s.baseURL != "" {
		return s.baseURL
	}
	return pythonFTPBaseURL
}

// FetchVersions returns a job for every release from minWindowsPython up to
// firstSigstoreOnlyPython whose amd64 folder has all of pythonMSIComponents
// with their .asc signatures. Releases without Windows binaries (source-only
// security releases) or without signatures (3.5.2) get no job.
func (s *PythonMSISource) FetchVersions() ([]MirrorJob, error) {
	verifier, err := newPythonWindowsVerifier()
	if err != nil {
		return nil, err
	}

	base := s.ftpBase()
	index, err := fetchText(base)
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", base, err)
	}

	var candidates []string
	for _, v := range parsePythonFTPVersions(index) {
		parts, _ := parseVersion(v)
		if compareVersions(parts, minWindowsPython) >= 0 && compareVersions(parts, firstSigstoreOnlyPython) < 0 {
			candidates = append(candidates, v)
		}
	}

	signed := make([]bool, len(candidates))
	var wg sync.WaitGroup
	sem := make(chan struct{}, msiDiscoveryWorkers)
	for i, v := range candidates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			listing, err := fetchText(base + v + "/amd64/")
			signed[i] = err == nil && hasSignedMSIs(listing)
		}()
	}
	wg.Wait()

	var jobs []MirrorJob
	for i, v := range candidates {
		if !signed[i] {
			continue
		}
		dirURL := base + v + "/amd64/"
		jobs = append(jobs, MirrorJob{
			Runtime:  "python",
			Version:  v,
			Platform: "windows-amd64",
			URL:      dirURL,
			R2Key:    fmt.Sprintf("python/%s/windows-amd64.tar.gz", v),
			MetaKey:  fmt.Sprintf("python/%s/windows-amd64.meta.json", v),
			Build: func() ([]byte, error) {
				return buildFromMSIs(dirURL, verifier, s.extract)
			},
		})
	}
	return jobs, nil
}

// parsePythonFTPVersions returns the x.y.z release folders in python.org's
// FTP index.
func parsePythonFTPVersions(index string) []string {
	var versions []string
	for _, m := range pythonFTPVersionPattern.FindAllStringSubmatch(index, -1) {
		versions = append(versions, m[1])
	}
	return versions
}

// hasSignedMSIs reports whether a release's amd64 folder listing has every
// one of pythonMSIComponents together with its .asc signature.
func hasSignedMSIs(listing string) bool {
	for _, c := range pythonMSIComponents {
		if !strings.Contains(listing, `href="`+c+`.msi"`) || !strings.Contains(listing, `href="`+c+`.msi.asc"`) {
			return false
		}
	}
	return true
}

// buildFromMSIs downloads pythonMSIComponents from dirURL, verifies each
// against its signature, unpacks them into one install and packs it.
func buildFromMSIs(dirURL string, verifier *signatureVerifier, extract func(msiPath, destDir string) error) ([]byte, error) {
	if extract == nil {
		extract = runMsiextract
	}

	work, err := os.MkdirTemp("", "dtvem-python-msi-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(work) }()

	installDir := filepath.Join(work, "install")
	for _, c := range pythonMSIComponents {
		msi, err := download(dirURL + c + ".msi")
		if err != nil {
			return nil, err
		}
		sig, err := download(dirURL + c + ".msi.asc")
		if err != nil {
			return nil, err
		}
		if err := verifier.verify(msi, sig); err != nil {
			return nil, fmt.Errorf("%s.msi: %w", c, err)
		}

		msiPath := filepath.Join(work, c+".msi")
		if err := os.WriteFile(msiPath, msi, 0o644); err != nil {
			return nil, err
		}
		if err := extract(msiPath, installDir); err != nil {
			return nil, fmt.Errorf("extracting %s.msi: %w", c, err)
		}
	}

	return archiveDir(installDir, true)
}

// runMsiextract unpacks an MSI with msiextract from msitools.
func runMsiextract(msiPath, destDir string) error {
	if _, err := exec.LookPath("msiextract"); err != nil {
		return errors.New("msiextract not found; install msitools (apt-get install msitools)")
	}
	out, err := exec.Command("msiextract", "-C", destDir, msiPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// PythonNuGetSource repackages python.org's NuGet packages, which hold a
// full install under tools/.
type PythonNuGetSource struct {
	// flatContainerURL and registrationURL point at nuget.org's API; tests
	// point them at a local server.
	flatContainerURL string
	registrationURL  string
}

func (s *PythonNuGetSource) Name() string {
	return "python.org NuGet packages"
}

// FetchVersions returns a job for every release package from
// minWindowsPython onward. Sources registered earlier win for any version
// and platform they also provide.
func (s *PythonNuGetSource) FetchVersions() ([]MirrorJob, error) {
	flat := s.flatContainerURL
	if flat == "" {
		flat = nugetFlatContainer
	}
	registration := s.registrationURL
	if registration == "" {
		registration = nugetRegistrationURL
	}

	var jobs []MirrorJob
	for _, pkg := range pythonNuGetPackages {
		body, err := download(flat + pkg.id + "/index.json")
		if err != nil {
			return nil, fmt.Errorf("listing %s: %w", pkg.id, err)
		}
		var index struct {
			Versions []string `json:"versions"`
		}
		if err := json.Unmarshal(body, &index); err != nil {
			return nil, fmt.Errorf("parsing %s index: %w", pkg.id, err)
		}

		for _, v := range index.Versions {
			parts, ok := parseVersion(v)
			if !ok || compareVersions(parts, minWindowsPython) < 0 {
				continue
			}
			id := pkg.id
			nupkgURL := fmt.Sprintf("%s%s/%s/%s.%s.nupkg", flat, id, v, id, v)
			leafURL := fmt.Sprintf("%s%s/%s.json", registration, id, v)
			jobs = append(jobs, MirrorJob{
				Runtime:  "python",
				Version:  v,
				Platform: pkg.platform,
				URL:      nupkgURL,
				R2Key:    fmt.Sprintf("python/%s/%s.tar.gz", v, pkg.platform),
				MetaKey:  fmt.Sprintf("python/%s/%s.meta.json", v, pkg.platform),
				Build: func() ([]byte, error) {
					return buildFromNuGet(nupkgURL, leafURL)
				},
			})
		}
	}
	return jobs, nil
}

// buildFromNuGet downloads a package, verifies it against the SHA-512 in
// its nuget.org catalog entry and repackages its tools/ folder.
func buildFromNuGet(nupkgURL, leafURL string) ([]byte, error) {
	wantHash, err := nugetPackageHash(leafURL)
	if err != nil {
		return nil, err
	}
	pkg, err := download(nupkgURL)
	if err != nil {
		return nil, err
	}
	if err := verifySHA512(pkg, wantHash); err != nil {
		return nil, fmt.Errorf("%s: %w", nupkgURL, err)
	}
	return archiveNuGetTools(pkg)
}

// nugetPackageHash returns the base64 SHA-512 nuget.org records for a
// package, read from the catalog entry its registration leaf points to.
func nugetPackageHash(leafURL string) (string, error) {
	body, err := download(leafURL)
	if err != nil {
		return "", err
	}
	var leaf struct {
		CatalogEntry string `json:"catalogEntry"`
	}
	if err := json.Unmarshal(body, &leaf); err != nil || leaf.CatalogEntry == "" {
		return "", fmt.Errorf("no catalog entry in %s", leafURL)
	}

	body, err = download(leaf.CatalogEntry)
	if err != nil {
		return "", err
	}
	var entry struct {
		PackageHash          string `json:"packageHash"`
		PackageHashAlgorithm string `json:"packageHashAlgorithm"`
	}
	if err := json.Unmarshal(body, &entry); err != nil {
		return "", fmt.Errorf("parsing %s: %w", leaf.CatalogEntry, err)
	}
	if !strings.EqualFold(entry.PackageHashAlgorithm, "SHA512") || entry.PackageHash == "" {
		return "", fmt.Errorf("%s has no SHA512 package hash", leaf.CatalogEntry)
	}
	return entry.PackageHash, nil
}

// verifySHA512 checks data against a base64-encoded SHA-512 digest.
func verifySHA512(data []byte, wantBase64 string) error {
	sum := sha512.Sum512(data)
	if got := base64.StdEncoding.EncodeToString(sum[:]); got != wantBase64 {
		return fmt.Errorf("SHA-512 mismatch: expected %s, got %s", wantBase64, got)
	}
	return nil
}

// signatureVerifier checks detached OpenPGP signatures against one pinned
// key.
type signatureVerifier struct {
	keyring     openpgp.EntityList
	fingerprint string
}

// newPythonWindowsVerifier returns a verifier for python.org's Windows
// release signing key.
func newPythonWindowsVerifier() (*signatureVerifier, error) {
	return newSignatureVerifier(pythonWindowsSigningKey, pythonWindowsSigningFingerprint)
}

// newSignatureVerifier parses an armored public key and confirms it is the
// key with the given fingerprint.
func newSignatureVerifier(armoredKey []byte, fingerprint string) (*signatureVerifier, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(armoredKey))
	if err != nil {
		return nil, fmt.Errorf("reading signing key: %w", err)
	}
	if len(keyring) != 1 {
		return nil, fmt.Errorf("signing key file holds %d keys, want 1", len(keyring))
	}
	if got := strings.ToUpper(hex.EncodeToString(keyring[0].PrimaryKey.Fingerprint)); got != fingerprint {
		return nil, fmt.Errorf("signing key fingerprint is %s, want %s", got, fingerprint)
	}
	return &signatureVerifier{keyring: keyring, fingerprint: fingerprint}, nil
}

// verify checks that armoredSig is a valid signature of data by the pinned
// key.
func (v *signatureVerifier) verify(data, armoredSig []byte) error {
	signer, err := openpgp.CheckArmoredDetachedSignature(v.keyring, bytes.NewReader(data), bytes.NewReader(armoredSig), nil)
	if err != nil {
		return fmt.Errorf("signature check failed: %w", err)
	}
	if got := strings.ToUpper(hex.EncodeToString(signer.PrimaryKey.Fingerprint)); got != v.fingerprint {
		return fmt.Errorf("signed by %s, want %s", got, v.fingerprint)
	}
	return nil
}

// archiveDir packs a full Windows Python install from dir. requireTk also
// demands tkinter, which MSI builds include and NuGet builds do not.
func archiveDir(dir string, requireTk bool) ([]byte, error) {
	var files []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() {
			rel, err := filepath.Rel(dir, p)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		} else if !d.IsDir() {
			return fmt.Errorf("unexpected non-regular file %s", p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := validateWindowsPython(files, requireTk); err != nil {
		return nil, err
	}

	w := newPythonArchive()
	sort.Strings(files)
	for _, rel := range files {
		data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
		if err != nil {
			return nil, err
		}
		if err := w.addFile(rel, data); err != nil {
			return nil, err
		}
	}
	return w.close()
}

// archiveNuGetTools packs the full install under tools/ in a NuGet package,
// dropping the package's own files.
func archiveNuGetTools(pkg []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return nil, fmt.Errorf("reading package: %w", err)
	}

	contents := make(map[string]*zip.File)
	var files []string
	for _, f := range zr.File {
		name := f.Name
		// NuGet stores part names URI-escaped ("script%20(dev).tmpl").
		if unescaped, err := url.PathUnescape(name); err == nil {
			name = unescaped
		}
		rel, ok := strings.CutPrefix(name, "tools/")
		if !ok || rel == "" || strings.HasSuffix(rel, "/") {
			continue
		}
		if !isSafeRelPath(rel) {
			return nil, fmt.Errorf("unsafe path in package: %s", f.Name)
		}
		contents[rel] = f
		files = append(files, rel)
	}
	if err := validateWindowsPython(files, false); err != nil {
		return nil, err
	}

	w := newPythonArchive()
	sort.Strings(files)
	for _, rel := range files {
		data, err := readZipFile(contents[rel])
		if err != nil {
			return nil, err
		}
		if err := w.addFile(rel, data); err != nil {
			return nil, err
		}
	}
	return w.close()
}

func readZipFile(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer func() { _ = rc.Close() }()
	return io.ReadAll(rc)
}

// isSafeRelPath reports whether p stays inside the directory it is
// relative to.
func isSafeRelPath(p string) bool {
	clean := path.Clean(p)
	return clean != "." && !path.IsAbs(clean) && clean != ".." && !strings.HasPrefix(clean, "../") && !strings.Contains(clean, ":")
}

// validateWindowsPython checks that files, relative to an install root,
// make a full Windows Python rather than an embeddable one.
func validateWindowsPython(files []string, requireTk bool) error {
	have := make(map[string]bool, len(files))
	importLib := false
	for _, f := range files {
		have[f] = true
		if strings.HasSuffix(f, "._pth") {
			return fmt.Errorf("%w: found %s, which would put it in isolated mode", errNotFullBuild, f)
		}
		if strings.HasPrefix(f, "libs/python") && strings.HasSuffix(f, ".lib") {
			importLib = true
		}
	}

	required := []string{"python.exe", "Lib/os.py", "Lib/venv/__init__.py", "Lib/ensurepip/__init__.py", "include/Python.h"}
	if requireTk {
		required = append(required, "DLLs/_tkinter.pyd", "Lib/tkinter/__init__.py")
	}
	var missing []string
	for _, r := range required {
		if !have[r] {
			missing = append(missing, r)
		}
	}
	if !importLib {
		missing = append(missing, "libs/pythonXY.lib")
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w, missing: %s", errNotFullBuild, strings.Join(missing, ", "))
	}
	return nil
}

// pythonArchive writes a .tar.gz with every entry under python/, the layout
// of python-build-standalone's Windows archives.
type pythonArchive struct {
	buf  bytes.Buffer
	gz   *gzip.Writer
	tw   *tar.Writer
	dirs map[string]bool
}

func newPythonArchive() *pythonArchive {
	a := &pythonArchive{dirs: make(map[string]bool)}
	a.gz = gzip.NewWriter(&a.buf)
	a.tw = tar.NewWriter(a.gz)
	return a
}

// addFile adds rel, a slash-separated path below the install root, after
// any of its parent directories not yet written.
func (a *pythonArchive) addFile(rel string, data []byte) error {
	if err := a.addDir(path.Dir(rel)); err != nil {
		return err
	}
	if err := a.tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "python/" + rel,
		Mode:     0o644,
		Size:     int64(len(data)),
		ModTime:  archiveModTime,
		Format:   tar.FormatPAX,
	}); err != nil {
		return err
	}
	_, err := a.tw.Write(data)
	return err
}

func (a *pythonArchive) addDir(rel string) error {
	if a.dirs[rel] {
		return nil
	}
	name := "python/"
	if rel != "." {
		if err := a.addDir(path.Dir(rel)); err != nil {
			return err
		}
		name += rel + "/"
	}
	a.dirs[rel] = true
	return a.tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeDir,
		Name:     name,
		Mode:     0o755,
		ModTime:  archiveModTime,
		Format:   tar.FormatPAX,
	})
}

func (a *pythonArchive) close() ([]byte, error) {
	if err := a.tw.Close(); err != nil {
		return nil, err
	}
	if err := a.gz.Close(); err != nil {
		return nil, err
	}
	return a.buf.Bytes(), nil
}

// download fetches url and returns its body, failing on any non-200 status.
func download(url string) ([]byte, error) {
	resp, err := httpGetWithRetry(url, 3)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("downloading %s: HTTP %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func fetchText(url string) (string, error) {
	body, err := download(url)
	return string(body), err
}

// parseVersion parses an x.y.z release version.
func parseVersion(v string) ([3]int, bool) {
	var parts [3]int
	if !releaseVersionPattern.MatchString(v) {
		return parts, false
	}
	for i, f := range strings.Split(v, ".") {
		n, err := strconv.Atoi(f)
		if err != nil {
			return parts, false
		}
		parts[i] = n
	}
	return parts, true
}

func compareVersions(a, b [3]int) int {
	for i := range a {
		if a[i] != b[i] {
			return a[i] - b[i]
		}
	}
	return 0
}
