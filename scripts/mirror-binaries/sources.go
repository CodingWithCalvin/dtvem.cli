package main

import (
	"errors"
	"fmt"
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

// httpGetWithRetry performs an HTTP GET with retries for transient failures
func httpGetWithRetry(url string, maxRetries int) (*http.Response, error) {
	req, err := newRequest(url)
	if err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
			continue
		}

		// Retry on server errors (5xx)
		if resp.StatusCode >= 500 {
			resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt) * 2 * time.Second)
			}
			continue
		}

		return resp, nil
	}
	return nil, lastErr
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
			&PythonOfficialSource{},
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
func collectJobs(sources []UpstreamSource) ([]MirrorJob, error) {
	var allJobs []MirrorJob
	var errs []error
	seen := make(map[string]bool) // Track version+platform to avoid duplicates

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
			key := fmt.Sprintf("%s/%s", job.Version, job.Platform)
			if !seen[key] {
				seen[key] = true
				allJobs = append(allJobs, job)
				added++
			}
		}
		fmt.Printf("  Found %d versions from %s (%d new)\n", len(jobs), source.Name(), added)
	}

	return allJobs, errors.Join(errs...)
}
