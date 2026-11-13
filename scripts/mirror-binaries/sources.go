package main

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// UpstreamSource represents a source of runtime binaries
type UpstreamSource interface {
	// Name returns a human-readable name for the source
	Name() string
	// FetchVersions fetches all available versions and their download info
	FetchVersions() ([]MirrorJob, error)
}

// httpClient is a shared HTTP client with reasonable timeouts
var httpClient = &http.Client{
	Timeout: 60 * time.Second,
}

// newRequest builds a GET request, authenticating GitHub API calls with
// GITHUB_TOKEN when it is set. Unauthenticated calls share a 60 requests/hour
// limit per IP, which CI runners regularly exhaust.
func newRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	// Only send the token to the GitHub API, never to arbitrary download hosts
	if req.URL.Host == "api.github.com" {
		req.Header.Set("Accept", "application/vnd.github+json")
		if token := os.Getenv("GITHUB_TOKEN"); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
	}

	return req, nil
}

// fetchedResponse is an HTTP response whose body has been read in full.
type fetchedResponse struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// retryDelay is the wait before the second attempt of a request. Each later
// attempt waits one retryDelay longer than the one before.
var retryDelay = 2 * time.Second

// fetchWithRetry performs an HTTP GET and reads the whole body. It tries again
// when the request fails, when the body is cut off part way, or when the
// server answers 5xx; any other status is returned for the caller to check.
//
// Reading the body inside the retry matters for GitHub's releases API, whose
// pages run to tens of megabytes: GitHub sometimes drops one part way through
// ("stream error: stream ID 13; CANCEL; received from peer").
func fetchWithRetry(url string, maxRetries int) (*fetchedResponse, error) {
	req, err := newRequest(url)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if attempt > 1 {
			time.Sleep(time.Duration(attempt-1) * retryDelay)
		}

		resp, err := fetchOnce(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			continue
		}

		return resp, nil
	}
	return nil, fmt.Errorf("after %d attempts: %w", maxRetries, lastErr)
}

// fetchOnce sends req and reads the whole response body.
func fetchOnce(req *http.Request) (*fetchedResponse, error) {
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &fetchedResponse{StatusCode: resp.StatusCode, Header: resp.Header, Body: body}, nil
}

// getUpstreamSources returns all upstream sources for a given runtime
func getUpstreamSources(runtime string) ([]UpstreamSource, error) {
	switch runtime {
	case "node":
		return []UpstreamSource{
			&NodeOfficialSource{},
		}, nil
	case "python":
		return []UpstreamSource{
			&PythonStandaloneSource{},
			// python.org's full Windows builds, published only at the
			// python.org location, for `dtvem install --signed`. The default
			// location is python-build-standalone's alone.
			&PythonMSISource{},
			&PythonNuGetSource{},
			&PythonInstallManagerSource{},
		}, nil
	case "ruby":
		return []UpstreamSource{
			&RubyInstallerSource{},
			&RubyBuilderSource{},
		}, nil
	default:
		return nil, fmt.Errorf("unknown runtime: %s", runtime)
	}
}

// fetchJobsFromUpstream fetches all mirror jobs for a runtime from upstream sources
func fetchJobsFromUpstream(runtime string) ([]MirrorJob, error) {
	sources, err := getUpstreamSources(runtime)
	if err != nil {
		return nil, err
	}

	return collectJobs(sources)
}

// collectJobs fetches mirror jobs from each source in order. A failing source
// does not stop the others: jobs from the sources that succeeded are returned
// together with an error describing every source that failed.
//
// When several jobs publish to the same place (the same metadata key), the
// first source's job wins. A version's default build is at
// <runtime>/<version>/<platform>; python.org's builds are at a place of
// their own, so they never compete with python-build-standalone's.
func collectJobs(sources []UpstreamSource) ([]MirrorJob, error) {
	var allJobs []MirrorJob
	var errs []error
	seen := make(map[string]bool) // Metadata keys already claimed

	for _, source := range sources {
		fmt.Printf("  Fetching from %s...\n", source.Name())
		jobs, err := source.FetchVersions()
		if err != nil {
			fmt.Printf("  Warning: failed to fetch from %s: %v\n", source.Name(), err)
			errs = append(errs, fmt.Errorf("%s: %w", source.Name(), err))
			continue
		}

		// Add jobs, avoiding duplicates (first source wins)
		added := 0
		for _, job := range jobs {
			if !seen[job.MetaKey] {
				seen[job.MetaKey] = true
				allJobs = append(allJobs, job)
				added++
			}
		}
		fmt.Printf("  Found %d jobs from %s (%d new)\n", len(jobs), source.Name(), added)
	}

	return allJobs, errors.Join(errs...)
}
