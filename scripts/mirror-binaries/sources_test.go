package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNewRequest(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		token     string
		wantAuth  string
		wantGHAPI bool
	}{
		{
			name:      "GitHub API with token",
			url:       "https://api.github.com/repos/astral-sh/python-build-standalone/releases",
			token:     "secret",
			wantAuth:  "Bearer secret",
			wantGHAPI: true,
		},
		{
			name:      "GitHub API without token",
			url:       "https://api.github.com/repos/astral-sh/python-build-standalone/releases",
			wantGHAPI: true,
		},
		{
			name:  "download host never receives token",
			url:   "https://github.com/astral-sh/python-build-standalone/releases/download/20261003/SHA256SUMS",
			token: "secret",
		},
		{
			name:  "unrelated host never receives token",
			url:   "https://nodejs.org/dist/index.json",
			token: "secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.token)

			req, err := newRequest(tt.url)
			if err != nil {
				t.Fatalf("newRequest(%q) error = %v, want nil", tt.url, err)
			}

			if got := req.Header.Get("Authorization"); got != tt.wantAuth {
				t.Errorf("Authorization header = %q, want %q", got, tt.wantAuth)
			}

			gotGHAPI := req.Header.Get("Accept") == "application/vnd.github+json"
			if gotGHAPI != tt.wantGHAPI {
				t.Errorf("GitHub Accept header set = %v, want %v", gotGHAPI, tt.wantGHAPI)
			}
		})
	}
}

func TestNewRequest_InvalidURL(t *testing.T) {
	if _, err := newRequest("://bad"); err == nil {
		t.Error("newRequest(\"://bad\") error = nil, want error")
	}
}

// noRetryDelay makes retries immediate for the rest of the test.
func noRetryDelay(t *testing.T) {
	t.Helper()

	old := retryDelay
	retryDelay = 0
	t.Cleanup(func() { retryDelay = old })
}

// writeCutOff starts a 200 response that promises more body than it sends and
// then drops the connection, as when GitHub cancels a stream part way.
func writeCutOff(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		t.Error("response writer cannot hijack the connection")
		return
	}
	conn, buf, err := hijacker.Hijack()
	if err != nil {
		t.Errorf("hijacking connection: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\n[{\"tag_name\":")
	_ = buf.Flush()
}

func TestFetchWithRetry(t *testing.T) {
	const body = `[{"tag_name":"20261113"}]`
	writeBody := func(w http.ResponseWriter) { _, _ = w.Write([]byte(body)) }

	tests := []struct {
		name string
		// respond answers the nth request (counting from 1).
		respond      func(t *testing.T, w http.ResponseWriter, n int32)
		wantStatus   int
		wantBody     string
		wantRequests int32
		wantErrText  string
	}{
		{
			name:         "complete on the first try",
			respond:      func(_ *testing.T, w http.ResponseWriter, _ int32) { writeBody(w) },
			wantStatus:   http.StatusOK,
			wantBody:     body,
			wantRequests: 1,
		},
		{
			name: "body cut off, then complete",
			respond: func(t *testing.T, w http.ResponseWriter, n int32) {
				if n == 1 {
					writeCutOff(t, w)
					return
				}
				writeBody(w)
			},
			wantStatus:   http.StatusOK,
			wantBody:     body,
			wantRequests: 2,
		},
		{
			name: "server error, then complete",
			respond: func(_ *testing.T, w http.ResponseWriter, n int32) {
				if n == 1 {
					http.Error(w, "bad gateway", http.StatusBadGateway)
					return
				}
				writeBody(w)
			},
			wantStatus:   http.StatusOK,
			wantBody:     body,
			wantRequests: 2,
		},
		{
			name: "client error returned without a retry",
			respond: func(_ *testing.T, w http.ResponseWriter, _ int32) {
				http.Error(w, "not found", http.StatusNotFound)
			},
			wantStatus:   http.StatusNotFound,
			wantBody:     "not found\n",
			wantRequests: 1,
		},
		{
			name:         "body cut off every time",
			respond:      func(t *testing.T, w http.ResponseWriter, _ int32) { writeCutOff(t, w) },
			wantRequests: 3,
			wantErrText:  "after 3 attempts: reading response: ",
		},
		{
			name: "server error every time",
			respond: func(_ *testing.T, w http.ResponseWriter, _ int32) {
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
			},
			wantRequests: 3,
			wantErrText:  "after 3 attempts: HTTP 503",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			noRetryDelay(t)

			var requests atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				tt.respond(t, w, requests.Add(1))
			}))
			t.Cleanup(srv.Close)

			resp, err := fetchWithRetry(srv.URL, 3)

			if got := requests.Load(); got != tt.wantRequests {
				t.Errorf("requests = %d, want %d", got, tt.wantRequests)
			}

			if tt.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("fetchWithRetry() error = %v, want error containing %q", err, tt.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("fetchWithRetry() error = %v, want nil", err)
			}
			if resp.StatusCode != tt.wantStatus {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
			if string(resp.Body) != tt.wantBody {
				t.Errorf("Body = %q, want %q", resp.Body, tt.wantBody)
			}
		})
	}
}

// fakeSource is an UpstreamSource returning canned jobs or an error.
type fakeSource struct {
	name string
	jobs []MirrorJob
	err  error
}

func (s *fakeSource) Name() string                        { return s.name }
func (s *fakeSource) FetchVersions() ([]MirrorJob, error) { return s.jobs, s.err }

func TestCollectJobs(t *testing.T) {
	primary := &fakeSource{
		name: "primary",
		jobs: []MirrorJob{
			{Version: "3.13.16", Platform: "windows-amd64", URL: "primary", MetaKey: "python/3.13.16/windows-amd64.meta.json"},
			{Version: "3.13.16", Platform: "linux-amd64", URL: "primary", MetaKey: "python/3.13.16/linux-amd64.meta.json"},
		},
	}
	fallback := &fakeSource{
		name: "fallback",
		jobs: []MirrorJob{
			{Version: "3.13.16", Platform: "windows-amd64", URL: "fallback", MetaKey: "python/3.13.16/windows-amd64.meta.json"},
			{Version: "3.11.17", Platform: "windows-amd64", URL: "fallback", MetaKey: "python/3.11.17/windows-amd64.meta.json"},
		},
	}
	// python.org's builds go to a place of their own, never a version's
	// default location.
	pythonOrg := &fakeSource{
		name: "python.org",
		jobs: []MirrorJob{pythonOrgJob("3.13.16", "windows-amd64", "python.org", nil)},
	}
	broken := &fakeSource{name: "broken", err: errors.New("fetching releases: HTTP 504")}

	tests := []struct {
		name        string
		sources     []UpstreamSource
		wantURLs    []string
		wantErrText string
	}{
		{
			name:     "first source wins on duplicates",
			sources:  []UpstreamSource{primary, fallback},
			wantURLs: []string{"primary", "primary", "fallback"},
		},
		{
			name:     "python.org build kept alongside the default",
			sources:  []UpstreamSource{primary, pythonOrg},
			wantURLs: []string{"primary", "primary", "python.org"},
		},
		{
			name:     "python.org build never takes the default location",
			sources:  []UpstreamSource{pythonOrg, primary},
			wantURLs: []string{"python.org", "primary", "primary"},
		},
		{
			// A failed python-build-standalone listing must not let a
			// python.org build take a default location, where a sync would
			// never replace it.
			name:        "python.org build stays in place when the default source fails",
			sources:     []UpstreamSource{broken, pythonOrg},
			wantURLs:    []string{"python.org"},
			wantErrText: "broken: fetching releases: HTTP 504",
		},
		{
			name:        "failing source keeps jobs from healthy sources",
			sources:     []UpstreamSource{broken, fallback},
			wantURLs:    []string{"fallback", "fallback"},
			wantErrText: "broken: fetching releases: HTTP 504",
		},
		{
			name:        "only source fails",
			sources:     []UpstreamSource{broken},
			wantErrText: "broken: fetching releases: HTTP 504",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jobs, err := collectJobs(tt.sources)

			if tt.wantErrText == "" {
				if err != nil {
					t.Errorf("collectJobs() error = %v, want nil", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErrText) {
				t.Errorf("collectJobs() error = %v, want error containing %q", err, tt.wantErrText)
			}

			var gotURLs []string
			for _, job := range jobs {
				gotURLs = append(gotURLs, job.URL)
			}
			if strings.Join(gotURLs, ",") != strings.Join(tt.wantURLs, ",") {
				t.Errorf("collectJobs() job URLs = %v, want %v", gotURLs, tt.wantURLs)
			}
		})
	}
}

// python-build-standalone fills the default location. python.org's MSIs
// (full x64 builds with tkinter, up to 3.13), NuGet packages (arm64) and
// install-manager zips (x64 from 3.14 on) fill the python.org location; for
// a version and platform two of them offer, the first wins.
func TestGetUpstreamSources_PythonOrder(t *testing.T) {
	sources, err := getUpstreamSources("python")
	if err != nil {
		t.Fatalf("getUpstreamSources(python) error: %v", err)
	}

	var got []string
	for _, s := range sources {
		got = append(got, s.Name())
	}
	want := []string{"python-build-standalone", "python.org MSIs", "python.org NuGet packages", "python.org install manager"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("python sources = %v, want %v", got, want)
	}
}
