package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestPendingJobs(t *testing.T) {
	build := func() ([]byte, error) { return nil, nil }
	job := func(version string, assembled bool) MirrorJob {
		j := MirrorJob{
			Version: version,
			R2Key:   "python/" + version + "/windows-amd64.tar.gz",
			MetaKey: "python/" + version + "/windows-amd64.meta.json",
		}
		if assembled {
			j.Build = build
		}
		return j
	}
	jobs := []MirrorJob{
		job("3.12.3", false), // archive and metadata mirrored
		job("3.9.11", false), // nothing mirrored yet
		job("3.8.20", false), // mirrored, but built from source
		job("3.7.9", false),  // metadata only: not a dtvem build, left alone
		job("3.9.9", true),   // dtvem build; metadata from the old embeddable .zip only
		job("3.12.2", true),  // dtvem build, archive and metadata mirrored
	}
	existing := map[string]bool{
		"python/3.12.3/windows-amd64.tar.gz":    true,
		"python/3.12.3/windows-amd64.meta.json": true,
		"python/3.8.20/windows-amd64.tar.gz":    true,
		"python/3.8.20/windows-amd64.meta.json": true,
		"python/3.7.9/windows-amd64.meta.json":  true,
		"python/3.9.9/windows-amd64.zip":        true,
		"python/3.9.9/windows-amd64.meta.json":  true,
		"python/3.12.2/windows-amd64.tar.gz":    true,
		"python/3.12.2/windows-amd64.meta.json": true,
	}
	builtFromSource := map[string]bool{"python/3.8.20/windows-amd64.meta.json": true}

	var got []string
	for _, j := range pendingJobs(jobs, existing, builtFromSource) {
		got = append(got, j.Version)
	}
	if want := []string{"3.9.11", "3.8.20", "3.9.9"}; !reflect.DeepEqual(got, want) {
		t.Errorf("pendingJobs() = %v, want %v", got, want)
	}
}

func TestRecordResult(t *testing.T) {
	stats := &Stats{}
	job := MirrorJob{R2Key: "python/3.5.2/windows-arm64.tar.gz"}

	recordResult(stats, job, nil)
	recordResult(stats, job, fmt.Errorf("build failed: %w", errNotFullBuild))
	recordResult(stats, job, errors.New("upload failed"))

	if stats.Mirrored != 1 || stats.Skipped != 1 || stats.Failed != 1 {
		t.Errorf("recordResult() counted mirrored=%d skipped=%d failed=%d, want 1 each", stats.Mirrored, stats.Skipped, stats.Failed)
	}
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func TestFetchBinary_Build(t *testing.T) {
	archive := []byte("assembled archive")
	job := MirrorJob{
		URL: "https://www.python.org/ftp/python/3.9.11/amd64/",
		// A Build job's checksum is always dtvem's, even if one were set.
		UpstreamSHA256: "ignored",
		Build:          func() ([]byte, error) { return archive, nil },
	}

	body, checksum, source, err := fetchBinary(job, &Stats{})
	if err != nil {
		t.Fatalf("fetchBinary() error: %v", err)
	}
	if string(body) != string(archive) || checksum != sha256Hex(archive) || source != "dtvem" {
		t.Errorf("fetchBinary() = (%q, %s, %s), want (%q, %s, dtvem)", body, checksum, source, archive, sha256Hex(archive))
	}

	job.Build = func() ([]byte, error) { return nil, errors.New("bad signature") }
	if _, _, _, err := fetchBinary(job, &Stats{}); err == nil {
		t.Error("fetchBinary() with a failing Build = nil error, want error")
	}
}

func TestFetchBinary_Download(t *testing.T) {
	payload := []byte("upstream archive")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	tests := []struct {
		name       string
		upstream   string
		wantSource string
		wantErr    bool
	}{
		{name: "matching upstream checksum", upstream: sha256Hex(payload), wantSource: "upstream"},
		{name: "no upstream checksum", wantSource: "dtvem"},
		{name: "mismatched upstream checksum", upstream: sha256Hex([]byte("other")), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, checksum, source, err := fetchBinary(MirrorJob{URL: srv.URL, UpstreamSHA256: tt.upstream}, &Stats{})
			if tt.wantErr {
				if err == nil {
					t.Error("fetchBinary() = nil error, want checksum mismatch")
				}
				return
			}
			if err != nil || source != tt.wantSource || checksum != sha256Hex(payload) {
				t.Errorf("fetchBinary() = (%s, %s, %v), want (%s, %s, nil)", checksum, source, err, sha256Hex(payload), tt.wantSource)
			}
		})
	}
}
