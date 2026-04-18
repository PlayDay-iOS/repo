package release

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/testutil"
	"github.com/google/go-github/v84/github"
)

func TestPublishPool_UploadsNewAssets(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stableDir := filepath.Join(root, "pool", "stable", "main")
	if err := os.MkdirAll(stableDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <test@test.com>"},
		{Key: "Description", Value: "Test"},
	})
	if err := os.WriteFile(filepath.Join(stableDir, "test.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	uploaded := make(map[string]bool)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// GetReleaseByTag
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{
				ID:      github.Ptr(int64(1)),
				TagName: github.Ptr("pool-stable"),
			})
			return
		}
		// ListReleaseAssets — empty
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/1/assets") {
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{})
			return
		}
		// UploadReleaseAsset
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/releases/1/assets") {
			name := r.URL.Query().Get("name")
			mu.Lock()
			uploaded[name] = true
			mu.Unlock()
			json.NewEncoder(w).Encode(&github.ReleaseAsset{
				ID:   github.Ptr(int64(10)),
				Name: github.Ptr(name),
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)

	cfg := &config.RepoConfig{
		Suites:    []string{"stable"},
		Component: "main",
		Hosting:   config.HostingConfig{Owner: "org", Repo: "repo", TagPrefix: "pool-"},
	}

	if err := PublishPool(context.Background(), client, cfg, root); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !uploaded["test.deb"] {
		t.Error("test.deb should have been uploaded")
	}
}

func TestPublishPool_SkipsExistingAsset(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stableDir := filepath.Join(root, "pool", "stable", "main")
	if err := os.MkdirAll(stableDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <test@test.com>"},
		{Key: "Description", Value: "Test"},
	})
	if err := os.WriteFile(filepath.Join(stableDir, "test.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(filepath.Join(stableDir, "test.deb"))

	var uploadCalled atomic.Bool
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{
				ID:      github.Ptr(int64(1)),
				TagName: github.Ptr("pool-stable"),
			})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/1/assets") {
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{
				{ID: github.Ptr(int64(5)), Name: github.Ptr("test.deb"), Size: github.Ptr(int(fi.Size()))},
			})
			return
		}
		if r.Method == http.MethodPost {
			uploadCalled.Store(true)
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)

	cfg := &config.RepoConfig{
		Suites:    []string{"stable"},
		Component: "main",
		Hosting:   config.HostingConfig{Owner: "org", Repo: "repo", TagPrefix: "pool-"},
	}

	if err := PublishPool(context.Background(), client, cfg, root); err != nil {
		t.Fatal(err)
	}
	if uploadCalled.Load() {
		t.Error("should not upload when asset exists with matching size")
	}
}

func TestPublishPool_SymlinkDedup(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stableDir := filepath.Join(root, "pool", "stable", "main")
	betaDir := filepath.Join(root, "pool", "beta", "main")
	if err := os.MkdirAll(stableDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(betaDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <test@test.com>"},
		{Key: "Description", Value: "Test"},
	})
	if err := os.WriteFile(filepath.Join(stableDir, "test.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(stableDir, "test.deb"), filepath.Join(betaDir, "test.deb")); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	uploadsByRelease := make(map[string][]string) // tag -> []names

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/pool-stable") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{ID: github.Ptr(int64(1)), TagName: github.Ptr("pool-stable")})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/pool-beta") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{ID: github.Ptr(int64(2)), TagName: github.Ptr("pool-beta")})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/assets") {
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{})
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/releases/") {
			name := r.URL.Query().Get("name")
			tag := "unknown"
			if strings.Contains(r.URL.Path, "/1/") {
				tag = "pool-stable"
			} else if strings.Contains(r.URL.Path, "/2/") {
				tag = "pool-beta"
			}
			mu.Lock()
			uploadsByRelease[tag] = append(uploadsByRelease[tag], name)
			mu.Unlock()
			json.NewEncoder(w).Encode(&github.ReleaseAsset{ID: github.Ptr(int64(10)), Name: github.Ptr(name)})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	client := newTestClient(t, handler)

	cfg := &config.RepoConfig{
		Suites:    []string{"stable", "beta"},
		Component: "main",
		Hosting:   config.HostingConfig{Owner: "org", Repo: "repo", TagPrefix: "pool-"},
	}

	if err := PublishPool(context.Background(), client, cfg, root); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	// test.deb should be uploaded ONLY to pool-stable (canonical location)
	if len(uploadsByRelease["pool-stable"]) != 1 || uploadsByRelease["pool-stable"][0] != "test.deb" {
		t.Errorf("pool-stable uploads = %v, want [test.deb]", uploadsByRelease["pool-stable"])
	}
	if len(uploadsByRelease["pool-beta"]) != 0 {
		t.Errorf("pool-beta should have no uploads (symlink dedup), got %v", uploadsByRelease["pool-beta"])
	}
}
