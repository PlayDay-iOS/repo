package ghimport

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/fileutil"
	"github.com/google/go-github/v84/github"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestCollectDebAssets_FiltersCorrectly(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{
			TagName:    github.Ptr("v1.0"),
			Draft:      github.Ptr(false),
			Prerelease: github.Ptr(false),
			Assets: []*github.ReleaseAsset{
				{Name: github.Ptr("tweak.deb"), BrowserDownloadURL: github.Ptr("https://example.com/tweak.deb")},
				{Name: github.Ptr("readme.md"), BrowserDownloadURL: github.Ptr("https://example.com/readme.md")},
			},
		},
		{
			TagName:    github.Ptr("v2.0-beta"),
			Draft:      github.Ptr(false),
			Prerelease: github.Ptr(true),
			Assets: []*github.ReleaseAsset{
				{Name: github.Ptr("tweak-beta.deb"), BrowserDownloadURL: github.Ptr("https://example.com/tweak-beta.deb")},
			},
		},
		{
			TagName:    github.Ptr("v0.1"),
			Draft:      github.Ptr(true),
			Prerelease: github.Ptr(false),
			Assets: []*github.ReleaseAsset{
				{Name: github.Ptr("draft.deb"), BrowserDownloadURL: github.Ptr("https://example.com/draft.deb")},
			},
		},
	}

	// Without prereleases
	assets := collectDebAssets(releases, false)
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset (no prereleases), got %d", len(assets))
	}
	if assets[0].name != "tweak.deb" {
		t.Errorf("expected tweak.deb, got %q", assets[0].name)
	}

	// With prereleases
	assets = collectDebAssets(releases, true)
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets (with prereleases), got %d", len(assets))
	}
}

func TestCollectDebAssets_SkipsDrafts(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{
			TagName: github.Ptr("v1.0"),
			Draft:   github.Ptr(true),
			Assets: []*github.ReleaseAsset{
				{Name: github.Ptr("pkg.deb"), BrowserDownloadURL: github.Ptr("https://example.com/pkg.deb")},
			},
		},
	}
	assets := collectDebAssets(releases, true)
	if len(assets) != 0 {
		t.Errorf("drafts should be skipped, got %d assets", len(assets))
	}
}

func TestCollectDebAssets_SkipsEmptyURL(t *testing.T) {
	releases := []*github.RepositoryRelease{
		{
			TagName: github.Ptr("v1.0"),
			Draft:   github.Ptr(false),
			Assets: []*github.ReleaseAsset{
				{Name: github.Ptr("pkg.deb"), BrowserDownloadURL: github.Ptr("")},
			},
		},
	}
	assets := collectDebAssets(releases, false)
	if len(assets) != 0 {
		t.Errorf("empty URL should be skipped, got %d assets", len(assets))
	}
}

func TestContainsPathChars(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"normal", false},
		{"com.example.pkg", false},
		{"single.dot", false},
		{"has..dots", false},
		{"has/slash", true},
		{"has\\backslash", true},
		{"has\x00null", true},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			if got := containsPathChars(tc.input); got != tc.want {
				t.Errorf("containsPathChars(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"normal.deb", "normal.deb"},
		{"has spaces.deb", "has-spaces.deb"},
		{"special@#$.deb", "special---.deb"},
		{"under_score-dash.deb", "under_score-dash.deb"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeFilename(tc.input)
			if got != tc.want {
				t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPlaceFile_FallsBackToCopyOnLinkError(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "in.deb")
	destPath := filepath.Join(tmpDir, "out.deb")
	if err := os.WriteFile(tmpPath, []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}

	placer := filePlacer{
		link: func(oldname, newname string) error {
			return os.ErrPermission
		},
		copyExclusive: fileutil.CopyFileExclusive,
	}

	result := placeFile(testLogger(), tmpPath, destPath, "out.deb", placer)
	if result != assetAdded {
		t.Fatalf("expected assetAdded, got %v", result)
	}
	if _, err := os.Stat(destPath); err != nil {
		t.Fatalf("expected destination file to exist: %v", err)
	}
}

func TestPlaceFile_ExistingDifferentContentRejected(t *testing.T) {
	tmpDir := t.TempDir()
	tmpPath := filepath.Join(tmpDir, "in.deb")
	destPath := filepath.Join(tmpDir, "out.deb")
	if err := os.WriteFile(tmpPath, []byte("new-content"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(destPath, []byte("old-content"), 0644); err != nil {
		t.Fatal(err)
	}

	result := placeFile(testLogger(), tmpPath, destPath, "out.deb", defaultPlacer)
	if result != assetRejected {
		t.Fatalf("expected assetRejected, got %v", result)
	}
}
