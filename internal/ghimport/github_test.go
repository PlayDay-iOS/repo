package ghimport

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-github/v84/github"
)


func TestNewGitHubClient_NoToken(t *testing.T) {
	client, err := NewGitHubClient("", "")
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewGitHubClient_WithToken(t *testing.T) {
	client, err := NewGitHubClient("test-token", "")
	if err != nil {
		t.Fatal(err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewGitHubClient_CustomBaseURL(t *testing.T) {
	client, err := NewGitHubClient("", "https://ghe.example.com/api/v3")
	if err != nil {
		t.Fatal(err)
	}
	if client.BaseURL.Host != "ghe.example.com" {
		t.Errorf("BaseURL host = %q, want ghe.example.com", client.BaseURL.Host)
	}
}

func TestFetchAllReleases_SinglePage(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{TagName: github.Ptr("v1.0"), Draft: github.Ptr(false)},
		{TagName: github.Ptr("v2.0"), Draft: github.Ptr(false)},
	}
	data, _ := json.Marshal(releases)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
	}))
	defer srv.Close()

	client, err := NewGitHubClient("", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FetchAllReleases(context.Background(), testLogger(), client, "org", "repo", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(got))
	}
	if got[0].GetTagName() != "v1.0" {
		t.Errorf("first tag = %q", got[0].GetTagName())
	}
}

func TestFetchAllReleases_Pagination(t *testing.T) {
	page1 := []*github.RepositoryRelease{
		{TagName: github.Ptr("v1.0"), Draft: github.Ptr(false)},
	}
	page2 := []*github.RepositoryRelease{
		{TagName: github.Ptr("v2.0"), Draft: github.Ptr(false)},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		pageParam := r.URL.Query().Get("page")
		if pageParam == "" || pageParam == "1" {
			// Link header pointing to page 2
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2&per_page=1>; rel="next"`)
			data, _ := json.Marshal(page1)
			w.Write(data)
		} else {
			data, _ := json.Marshal(page2)
			w.Write(data)
		}
	}))
	defer srv.Close()

	client, err := NewGitHubClient("", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	got, err := FetchAllReleases(context.Background(), testLogger(), client, "org", "repo", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 releases across 2 pages, got %d", len(got))
	}
	if got[0].GetTagName() != "v1.0" {
		t.Errorf("first tag = %q, want v1.0", got[0].GetTagName())
	}
	if got[1].GetTagName() != "v2.0" {
		t.Errorf("second tag = %q, want v2.0", got[1].GetTagName())
	}
}

func TestDownloadFile_Success(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("file-content"))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	if err := DownloadFile(srv.URL+"/test.deb", dst, srv.Client()); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(dst)
	if string(data) != "file-content" {
		t.Errorf("content = %q", string(data))
	}
}

func TestDownloadFile_RejectsNonHTTPSRedirect(t *testing.T) {
	// HTTP target that the TLS server redirects to
	httpTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("should not reach"))
	}))
	defer httpTarget.Close()

	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, httpTarget.URL+"/evil", http.StatusFound)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	err := DownloadFile(srv.URL+"/test.deb", dst, srv.Client())
	if err == nil {
		t.Fatal("expected error for non-HTTPS redirect")
	}
	if !strings.Contains(err.Error(), "non-HTTPS redirect") {
		t.Errorf("expected non-HTTPS redirect error, got: %v", err)
	}
}

func TestDownloadFile_RejectsHTTP(t *testing.T) {
	dst := filepath.Join(t.TempDir(), "out.bin")
	err := DownloadFile("http://example.com/test.deb", dst, nil)
	if err == nil {
		t.Fatal("expected error for HTTP URL")
	}
}

func TestDownloadFile_HTTPError(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "out.bin")
	err := DownloadFile(srv.URL+"/missing", dst, srv.Client())
	if err == nil {
		t.Fatal("expected error for 404")
	}
}
