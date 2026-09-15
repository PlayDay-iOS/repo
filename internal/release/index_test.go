package release

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/google/go-github/v84/github"
)

func TestPublishIndex_UploadsEveryFileAndReplacesStaleOnes(t *testing.T) {
	t.Parallel()
	indexDir := t.TempDir()
	suiteDir := filepath.Join(indexDir, "stable")
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Release", "Packages", "Packages.gz", "CydiaIcon.png"} {
		if err := os.WriteFile(filepath.Join(suiteDir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	var mu sync.Mutex
	var uploaded []string
	var deleted []int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/pool-stable") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{ID: github.Ptr(int64(1)), TagName: github.Ptr("pool-stable")})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/1/assets") {
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{
				{ID: github.Ptr(int64(7)), Name: github.Ptr("Packages"), Size: github.Ptr(1)},
			})
			return
		}
		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/releases/assets/") {
			mu.Lock()
			deleted = append(deleted, 7)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/assets") {
			name := r.URL.Query().Get("name")
			mu.Lock()
			uploaded = append(uploaded, name)
			mu.Unlock()
			json.NewEncoder(w).Encode(&github.ReleaseAsset{ID: github.Ptr(int64(10)), Name: github.Ptr(name)})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)

	cfg := &config.RepoConfig{
		Suites:  []string{"stable"},
		Hosting: config.HostingConfig{Owner: "org", Repo: "repo", TagPrefix: "pool-"},
	}

	if err := PublishIndex(context.Background(), client, cfg, indexDir); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	sort.Strings(uploaded)
	want := []string{"CydiaIcon.png", "Packages", "Packages.gz", "Release"}
	if strings.Join(uploaded, ",") != strings.Join(want, ",") {
		t.Errorf("uploaded = %v, want %v", uploaded, want)
	}
	// An index file with unchanged size is still stale content, so the existing
	// asset has to be deleted rather than skipped.
	if len(deleted) != 1 {
		t.Errorf("deleted = %v, want the pre-existing Packages asset", deleted)
	}
}

func TestPublishIndex_SkipsSuiteWithoutBuiltIndex(t *testing.T) {
	t.Parallel()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)

	cfg := &config.RepoConfig{
		Suites:  []string{"beta"},
		Hosting: config.HostingConfig{Owner: "org", Repo: "repo", TagPrefix: "pool-"},
	}

	if err := PublishIndex(context.Background(), client, cfg, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}
