package release

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
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

func TestPublishPool_SymlinkUploadsIntoEachSuite(t *testing.T) {
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
	// Each suite's index addresses its own release, so a symlinked deb needs a
	// copy in both rather than a single canonical upload.
	for _, tag := range []string{"pool-stable", "pool-beta"} {
		if len(uploadsByRelease[tag]) != 1 || uploadsByRelease[tag][0] != "test.deb" {
			t.Errorf("%s uploads = %v, want [test.deb]", tag, uploadsByRelease[tag])
		}
	}
}

func TestPublishPool_DuplicateNamesWithinSuiteUploadedOnce(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	stableDir := filepath.Join(root, "pool", "stable", "main")
	nestedDir := filepath.Join(stableDir, "nested")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
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
	if err := os.Symlink(filepath.Join(stableDir, "test.deb"), filepath.Join(nestedDir, "test.deb")); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var uploads []string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{ID: github.Ptr(int64(1)), TagName: github.Ptr("pool-stable")})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/assets") {
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{})
			return
		}
		if r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/assets") {
			name := r.URL.Query().Get("name")
			mu.Lock()
			uploads = append(uploads, name)
			mu.Unlock()
			json.NewEncoder(w).Encode(&github.ReleaseAsset{ID: github.Ptr(int64(10)), Name: github.Ptr(name)})
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
	// A release tag is one flat namespace, so the same asset name can only be
	// uploaded once no matter how many pool paths reach it.
	if len(uploads) != 1 {
		t.Errorf("uploads = %v, want exactly one test.deb", uploads)
	}
}

func TestPublishPool_PrunesStaleDebAssets(t *testing.T) {
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
	if err := os.WriteFile(filepath.Join(stableDir, "keep.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(filepath.Join(stableDir, "keep.deb"))
	if err != nil {
		t.Fatal(err)
	}

	// The index now shares the release with the payloads, so the prune must
	// leave every non-.deb asset alone.
	assets := []*github.ReleaseAsset{
		{ID: github.Ptr(int64(5)), Name: github.Ptr("keep.deb"), Size: github.Ptr(int(fi.Size()))},
		{ID: github.Ptr(int64(6)), Name: github.Ptr("stale.deb"), Size: github.Ptr(123)},
		{ID: github.Ptr(int64(7)), Name: github.Ptr("Packages"), Size: github.Ptr(123)},
		{ID: github.Ptr(int64(8)), Name: github.Ptr("Release"), Size: github.Ptr(123)},
		{ID: github.Ptr(int64(9)), Name: github.Ptr("CydiaIcon.png"), Size: github.Ptr(123)},
	}

	var mu sync.Mutex
	var deleted []string
	byID := map[string]string{"5": "keep.deb", "6": "stale.deb", "7": "Packages", "8": "Release", "9": "CydiaIcon.png"}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/releases/assets/") {
			id := path.Base(r.URL.Path)
			mu.Lock()
			deleted = append(deleted, byID[id])
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{ID: github.Ptr(int64(1)), TagName: github.Ptr("pool-stable")})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/1/assets") {
			json.NewEncoder(w).Encode(assets)
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
	if len(deleted) != 1 || deleted[0] != "stale.deb" {
		t.Errorf("deleted = %v, want [stale.deb]", deleted)
	}
}

func TestPublishPool_PrunesSuiteWithNoPackages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "pool", "stable", "main"), 0755); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	var deleted []int64

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/releases/assets/") {
			id, _ := strconv.ParseInt(path.Base(r.URL.Path), 10, 64)
			mu.Lock()
			deleted = append(deleted, id)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/tags/") {
			json.NewEncoder(w).Encode(&github.RepositoryRelease{ID: github.Ptr(int64(1)), TagName: github.Ptr("pool-stable")})
			return
		}
		if r.Method == http.MethodGet && strings.Contains(r.URL.Path, "/releases/1/assets") {
			json.NewEncoder(w).Encode([]*github.ReleaseAsset{
				{ID: github.Ptr(int64(6)), Name: github.Ptr("stale.deb"), Size: github.Ptr(123)},
				{ID: github.Ptr(int64(7)), Name: github.Ptr("Packages"), Size: github.Ptr(123)},
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
	// A suite whose pool went empty still has to lose its stale payloads.
	if len(deleted) != 1 || deleted[0] != 6 {
		t.Errorf("deleted = %v, want [6]", deleted)
	}
}
