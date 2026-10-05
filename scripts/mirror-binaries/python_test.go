package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestNextPageURL(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
	}{
		{
			name:   "next before last",
			header: `<https://api.github.com/r?per_page=10&page=2>; rel="next", <https://api.github.com/r?per_page=10&page=9>; rel="last"`,
			want:   "https://api.github.com/r?per_page=10&page=2",
		},
		{
			name:   "next after prev",
			header: `<https://api.github.com/r?page=1>; rel="prev", <https://api.github.com/r?page=3>; rel="next"`,
			want:   "https://api.github.com/r?page=3",
		},
		{
			name:   "last page has no next",
			header: `<https://api.github.com/r?page=1>; rel="first", <https://api.github.com/r?page=8>; rel="prev"`,
			want:   "",
		},
		{
			name:   "empty header",
			header: "",
			want:   "",
		},
		{
			name:   "target without angle brackets",
			header: `https://api.github.com/r?page=2; rel="next"`,
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPageURL(tt.header); got != tt.want {
				t.Errorf("nextPageURL(%q) = %q, want %q", tt.header, got, tt.want)
			}
		})
	}
}

// newReleasesServer serves total releases tagged "1".."total" (newest first),
// paginated like the GitHub releases API. If failPage is non-zero, that page
// responds with HTTP 403. The returned counter tracks requests served.
func newReleasesServer(t *testing.T, total, failPage int) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var requests atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)

		perPage, err := strconv.Atoi(r.URL.Query().Get("per_page"))
		if err != nil {
			http.Error(w, "missing per_page", http.StatusBadRequest)
			return
		}
		page := 1
		if p := r.URL.Query().Get("page"); p != "" {
			page, _ = strconv.Atoi(p)
		}

		if page == failPage {
			http.Error(w, "rate limited", http.StatusForbidden)
			return
		}

		start := (page - 1) * perPage
		end := min(start+perPage, total)
		var releases []githubRelease
		for i := start; i < end; i++ {
			releases = append(releases, githubRelease{TagName: strconv.Itoa(i + 1)})
		}

		if end < total {
			w.Header().Set("Link", fmt.Sprintf(`<http://%s%s?per_page=%d&page=%d>; rel="next"`,
				r.Host, r.URL.Path, perPage, page+1))
		}
		if err := json.NewEncoder(w).Encode(releases); err != nil {
			t.Errorf("encoding releases: %v", err)
		}
	}))
	t.Cleanup(srv.Close)

	return srv, &requests
}

func TestFetchGitHubReleases(t *testing.T) {
	tests := []struct {
		name         string
		total        int
		pageSize     int
		maxReleases  int
		wantReleases int
		wantRequests int32
	}{
		{
			name:         "follows every page",
			total:        25,
			pageSize:     10,
			maxReleases:  100,
			wantReleases: 25,
			wantRequests: 3,
		},
		{
			name:         "single page",
			total:        4,
			pageSize:     10,
			maxReleases:  100,
			wantReleases: 4,
			wantRequests: 1,
		},
		{
			name:         "stops at max releases",
			total:        50,
			pageSize:     10,
			maxReleases:  15,
			wantReleases: 15,
			wantRequests: 2,
		},
		{
			name:         "max releases on page boundary",
			total:        50,
			pageSize:     10,
			maxReleases:  20,
			wantReleases: 20,
			wantRequests: 2,
		},
		{
			name:         "no releases",
			total:        0,
			pageSize:     10,
			maxReleases:  100,
			wantReleases: 0,
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv, requests := newReleasesServer(t, tt.total, 0)

			releases, err := fetchGitHubReleases(srv.URL+"/releases", tt.pageSize, tt.maxReleases)
			if err != nil {
				t.Fatalf("fetchGitHubReleases() error = %v, want nil", err)
			}

			if got := len(releases); got != tt.wantReleases {
				t.Errorf("len(releases) = %d, want %d", got, tt.wantReleases)
			}
			if got := requests.Load(); got != tt.wantRequests {
				t.Errorf("requests = %d, want %d", got, tt.wantRequests)
			}

			// Releases must stay newest first across page boundaries
			for i, release := range releases {
				if want := strconv.Itoa(i + 1); release.TagName != want {
					t.Errorf("releases[%d].TagName = %q, want %q", i, release.TagName, want)
					break
				}
			}
		})
	}
}

func TestFetchGitHubReleases_PageError(t *testing.T) {
	srv, _ := newReleasesServer(t, 30, 2)

	releases, err := fetchGitHubReleases(srv.URL+"/releases", 10, 100)
	if err == nil {
		t.Fatalf("fetchGitHubReleases() error = nil, want error for failed page 2")
	}
	if releases != nil {
		t.Errorf("fetchGitHubReleases() releases = %v, want nil on error", releases)
	}

	want := "page 2: fetching releases: HTTP 403"
	if err.Error() != want {
		t.Errorf("fetchGitHubReleases() error = %q, want %q", err.Error(), want)
	}
}

func TestFetchGitHubReleases_MalformedPage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not json</html>"))
	}))
	t.Cleanup(srv.Close)

	_, err := fetchGitHubReleases(srv.URL+"/releases", 10, 100)
	if err == nil || !strings.HasPrefix(err.Error(), "page 1: parsing releases:") {
		t.Errorf("fetchGitHubReleases() error = %v, want page 1 parsing error", err)
	}
}
