package main

// python.org's builds of Python 3.14 and later for windows-amd64.
//
// From 3.14 on python.org signs its release files with Sigstore only, and
// its per-component MSIs carry no signature of their own, so PythonMSISource
// cannot verify them. python.org's Python install manager installs from
// full-install zips instead, listed with their SHA-256 in an index on
// python.org. This source repackages those zips, checked against that
// index, into the same python/ layout as the other Windows builds.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

const (
	// pythonInstallManagerIndexURL is the first page of the index the Python
	// install manager reads. Each page names the next in "next".
	pythonInstallManagerIndexURL = "https://www.python.org/ftp/python/index-windows.json"

	// maxInstallManagerIndexPages bounds how many "next" links are followed.
	maxInstallManagerIndexPages = 10
)

// installManagerAMD64ID matches the index id of the regular 64-bit x86
// build of a release ("pythoncore-3.14-64"), not the free-threaded ("3.14t")
// or 32-bit ones.
var installManagerAMD64ID = regexp.MustCompile(`^pythoncore-(\d+\.\d+)-64$`)

// PythonInstallManagerSource repackages python.org's install-manager zips
// for windows-amd64, from firstSigstoreOnlyPython on.
type PythonInstallManagerSource struct {
	// indexURL is the index's first page; tests point it at a local server.
	indexURL string
	// downloadPrefix is the only place zips may come from; tests point it
	// at a local server.
	downloadPrefix string
}

func (s *PythonInstallManagerSource) Name() string {
	return "python.org install manager"
}

// installManagerIndex is one page of the install manager's index.
type installManagerIndex struct {
	Versions []installManagerEntry `json:"versions"`
	Next     string                `json:"next"`
}

type installManagerEntry struct {
	ID          string `json:"id"`
	SortVersion string `json:"sort-version"`
	URL         string `json:"url"`
	Hash        struct {
		SHA256 string `json:"sha256"`
	} `json:"hash"`
}

// FetchVersions returns jobs for every x.y.z release from
// firstSigstoreOnlyPython on that the index lists for 64-bit x86, from
// python.org, with a SHA-256.
func (s *PythonInstallManagerSource) FetchVersions() ([]MirrorJob, error) {
	entries, err := s.fetchIndex()
	if err != nil {
		return nil, err
	}

	prefix := s.downloadPrefix
	if prefix == "" {
		prefix = pythonFTPBaseURL
	}

	var jobs []MirrorJob
	seen := make(map[string]bool)
	for _, e := range entries {
		m := installManagerAMD64ID.FindStringSubmatch(e.ID)
		if m == nil {
			continue
		}
		parts, ok := parseVersion(e.SortVersion)
		if !ok || compareVersions(parts, firstSigstoreOnlyPython) < 0 {
			continue
		}
		if !strings.HasPrefix(e.SortVersion, m[1]+".") {
			continue
		}
		if !strings.HasPrefix(e.URL, prefix) || !strings.HasSuffix(e.URL, ".zip") || !isSHA256Hex(e.Hash.SHA256) {
			continue
		}
		if seen[e.SortVersion] {
			continue
		}
		seen[e.SortVersion] = true

		zipURL, want := e.URL, strings.ToLower(e.Hash.SHA256)
		jobs = append(jobs, pythonOrgJobs(e.SortVersion, "windows-amd64", zipURL, func() ([]byte, error) {
			return buildFromInstallManagerZip(zipURL, want)
		})...)
	}
	return jobs, nil
}

// fetchIndex reads every page of the index, following "next" links.
func (s *PythonInstallManagerSource) fetchIndex() ([]installManagerEntry, error) {
	page := s.indexURL
	if page == "" {
		page = pythonInstallManagerIndexURL
	}

	var entries []installManagerEntry
	visited := make(map[string]bool)
	for page != "" {
		if visited[page] {
			return nil, fmt.Errorf("index page %s links back to itself", page)
		}
		if len(visited) == maxInstallManagerIndexPages {
			return nil, fmt.Errorf("index has more than %d pages", maxInstallManagerIndexPages)
		}
		visited[page] = true

		body, err := download(page)
		if err != nil {
			return nil, err
		}
		var index installManagerIndex
		if err := json.Unmarshal(body, &index); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", page, err)
		}
		entries = append(entries, index.Versions...)

		if index.Next == "" {
			break
		}
		base, err := url.Parse(page)
		if err != nil {
			return nil, err
		}
		next, err := base.Parse(index.Next)
		if err != nil {
			return nil, fmt.Errorf("next page %q in %s: %w", index.Next, page, err)
		}
		page = next.String()
	}
	return entries, nil
}

// buildFromInstallManagerZip downloads an install-manager zip, verifies it
// against the SHA-256 the index lists and repackages it.
func buildFromInstallManagerZip(zipURL, wantSHA256 string) ([]byte, error) {
	pkg, err := download(zipURL)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(pkg)
	if got := hex.EncodeToString(sum[:]); got != wantSHA256 {
		return nil, fmt.Errorf("%s: SHA-256 mismatch: expected %s, got %s", zipURL, wantSHA256, got)
	}
	// The zips hold tkinter but not the Tcl/Tk script library.
	return archiveZipTree(pkg, "", false)
}

// isSHA256Hex reports whether s is a hex-encoded SHA-256 digest.
func isSHA256Hex(s string) bool {
	if len(s) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
