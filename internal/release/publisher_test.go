package release

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-github/v84/github"
)

func init() {
	sleepFn = func(time.Duration) {} // no-op for tests
}

func newTestClient(t *testing.T, handler http.Handler) *github.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := github.NewClient(nil).WithEnterpriseURLs(srv.URL, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestEnsureRelease_ExistingRelease(t *testing.T) {
	t.Parallel()
	rel := &github.RepositoryRelease{
		ID:      github.Ptr(int64(42)),
		TagName: github.Ptr("pool-stable"),
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/releases/tags/pool-stable") {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(rel)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)
	p := NewPublisher(client, "owner", "repo")

	id, err := p.EnsureRelease(context.Background(), "pool-stable")
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Errorf("release ID = %d, want 42", id)
	}
}

func TestEnsureRelease_CreatesWhenMissing(t *testing.T) {
	t.Parallel()
	created := &github.RepositoryRelease{
		ID:      github.Ptr(int64(99)),
		TagName: github.Ptr("pool-stable"),
	}
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/releases") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(created)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)
	p := NewPublisher(client, "owner", "repo")

	id, err := p.EnsureRelease(context.Background(), "pool-stable")
	if err != nil {
		t.Fatal(err)
	}
	if id != 99 {
		t.Errorf("release ID = %d, want 99", id)
	}
}

func TestListAssets_Paginated(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		page := r.URL.Query().Get("page")
		if page == "" || page == "1" {
			w.Header().Set("Link", `<`+r.URL.Path+`?page=2&per_page=1>; rel="next"`)
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{
				{ID: github.Ptr(int64(1)), Name: github.Ptr("a.deb"), Size: github.Ptr(100)},
			})
		} else {
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{
				{ID: github.Ptr(int64(2)), Name: github.Ptr("b.deb"), Size: github.Ptr(200)},
			})
		}
	})
	client := newTestClient(t, handler)
	p := NewPublisher(client, "owner", "repo")

	assets, err := p.ListAssets(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}
	if assets["a.deb"].Size != 100 || assets["b.deb"].Size != 200 {
		t.Errorf("unexpected asset sizes: %+v", assets)
	}
}

func TestUploadAsset_Success(t *testing.T) {
	t.Parallel()
	var received []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/releases/42/assets") {
			received, _ = io.ReadAll(r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(&github.ReleaseAsset{
				ID:   github.Ptr(int64(10)),
				Name: github.Ptr("test.deb"),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)
	p := NewPublisher(client, "owner", "repo")

	f := filepath.Join(t.TempDir(), "test.deb")
	if err := os.WriteFile(f, []byte("deb-content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := p.UploadAsset(context.Background(), 42, "test.deb", f); err != nil {
		t.Fatal(err)
	}
	if string(received) != "deb-content" {
		t.Errorf("received = %q", string(received))
	}
}

func TestReplaceAsset_DeleteThenUpload(t *testing.T) {
	t.Parallel()
	var deleted atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/releases/assets/5") {
			deleted.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/releases/42/assets") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(&github.ReleaseAsset{
				ID:   github.Ptr(int64(10)),
				Name: github.Ptr("test.deb"),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)
	p := NewPublisher(client, "owner", "repo")

	f := filepath.Join(t.TempDir(), "test.deb")
	if err := os.WriteFile(f, []byte("new-content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := p.ReplaceAsset(context.Background(), 42, 5, "test.deb", f); err != nil {
		t.Fatal(err)
	}
	if !deleted.Load() {
		t.Error("old asset should have been deleted")
	}
}

func TestRetryOn5xx(t *testing.T) {
	t.Parallel()
	var attempts atomic.Int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if strings.Contains(r.URL.Path, "/releases/tags/") {
			if n < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(&github.RepositoryRelease{
				ID:      github.Ptr(int64(1)),
				TagName: github.Ptr("pool-stable"),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)
	p := NewPublisher(client, "owner", "repo")

	id, err := p.EnsureRelease(context.Background(), "pool-stable")
	if err != nil {
		t.Fatalf("expected retry to succeed, got: %v", err)
	}
	if id != 1 {
		t.Errorf("release ID = %d, want 1", id)
	}
	if got := attempts.Load(); got != 3 {
		t.Errorf("expected 3 attempts, got %d", got)
	}
}
