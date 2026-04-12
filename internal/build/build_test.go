package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PlayDay-iOS/repo/internal/testutil"
)

func TestRun_EmptyPool(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "_site")

	// Create pool dirs
	if err := os.MkdirAll(filepath.Join(root, "pool", "stable", "main"), 0755); err != nil {
		t.Fatal(err)
	}
	// Create config
	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create template
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "templates", "index.html.tmpl"),
		[]byte("<html><title>{{.RepoName}}</title></html>"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create icon
	if err := os.MkdirAll(filepath.Join(root, "resources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources", "CydiaIcon.png"), []byte("PNG"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Verify output structure
	for _, f := range []string{
		"index.html",
		"CydiaIcon.png",
		"stable/Packages",
		"stable/Packages.gz",
		"stable/Packages.xz",
		"stable/Packages.bz2",
		"stable/Release",
		"stable/CydiaIcon.png",
		"stable/index.html",
	} {
		if _, err := os.Stat(filepath.Join(output, f)); err != nil {
			t.Errorf("missing expected file: %s", f)
		}
	}
}

func TestRun_WithDeb(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "_site")

	poolDir := filepath.Join(root, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Write a test .deb
	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <t@t.com>"},
		{Key: "Description", Value: "Test package"},
	})
	if err := os.WriteFile(filepath.Join(poolDir, "test.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "templates", "index.html.tmpl"),
		[]byte("<html>{{.RepoName}}</html>"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Check Packages has content
	pkgData, err := os.ReadFile(filepath.Join(output, "stable", "Packages"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(pkgData)
	if !strings.Contains(content, "Package: com.test.pkg") {
		t.Error("Packages should contain the package")
	}
	if !strings.Contains(content, "Filename: pool/stable/main/test.deb") {
		t.Error("Packages should reference correct filename")
	}

	// Verify pool mirror exists
	mirrorPath := filepath.Join(output, "stable", "pool", "stable", "main", "test.deb")
	if _, err := os.Stat(mirrorPath); err != nil {
		t.Error("pool mirror should exist at stable/pool/stable/main/test.deb")
	}

	// Verify no orphaned top-level pool in output
	if _, err := os.Stat(filepath.Join(output, "pool")); !os.IsNotExist(err) {
		t.Error("top-level pool/ should not exist in output")
	}
}

func TestRun_RejectsDisallowedArchitectureFromPool(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "_site")

	poolDir := filepath.Join(root, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "amd64"},
		{Key: "Maintainer", Value: "Test <t@t.com>"},
		{Key: "Description", Value: "Test package"},
	})
	if err := os.WriteFile(filepath.Join(poolDir, "test.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "templates", "index.html.tmpl"),
		[]byte("<html>{{.RepoName}}</html>"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
	})
	if err == nil {
		t.Fatal("expected error for disallowed architecture")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error should mention disallowed architecture, got: %v", err)
	}
}

func TestLoadGPGKey_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.asc")
	if err := os.WriteFile(keyFile, []byte("file-key"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := loadGPGKey(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "file-key" {
		t.Errorf("expected file key, got %q", got)
	}
}

func TestLoadGPGKey_EmptyReturnsEmpty(t *testing.T) {
	got, err := loadGPGKey("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestLoadGPGKey_MissingFileErrors(t *testing.T) {
	_, err := loadGPGKey("/nonexistent/key.asc")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestRun_GPGKeyFromOptions(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "_site")

	// Write config with a gpg_key_file that doesn't exist
	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
[signing]
gpg_key_file = "/nonexistent/key.asc"
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "templates", "index.html.tmpl"),
		[]byte("<html>{{.RepoName}}</html>"), 0644); err != nil {
		t.Fatal(err)
	}

	// Options.GPGKey should take precedence over missing config file
	// Empty key means no signing, so it should succeed without reading the missing file
	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
		GPGKey:       "", // no key = no signing, but config has invalid path
	})
	// This should fail because config has a missing gpg_key_file and Options.GPGKey is empty
	if err == nil {
		t.Fatal("expected error for missing GPG key file")
	}

	// Now provide key via Options — should skip config file entirely
	err = Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
		GPGKey:       "fake-key-for-test", // will fail signing but proves precedence
	})
	// This will fail at signing (invalid key), but should NOT fail at loading
	if err != nil && strings.Contains(err.Error(), "GPG key file") {
		t.Errorf("should not try to read config gpg_key_file when Options.GPGKey is set, got: %v", err)
	}
}

func TestResolveBuildTime_Override(t *testing.T) {
	fixed := time.Date(2026, 1, 15, 12, 0, 0, 0, time.UTC)
	got := ResolveBuildTime(fixed)
	if !got.Equal(fixed) {
		t.Errorf("expected override time, got %v", got)
	}
}

func TestResolveBuildTime_FallsBackToNow(t *testing.T) {
	before := time.Now().UTC().Add(-time.Second)
	got := ResolveBuildTime(time.Time{})
	after := time.Now().UTC().Add(time.Second)
	if got.Before(before) || got.After(after) {
		t.Errorf("expected current time, got %v", got)
	}
}

func TestRun_MarkerSafety_RefuseWithoutMarker(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "_site")

	// Create output dir without marker
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}

	// Config must exist (loaded before marker check)
	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
	})
	if err == nil {
		t.Fatal("expected error when output dir exists without marker")
	}
	if !strings.Contains(err.Error(), "marker") {
		t.Errorf("error should mention marker, got: %v", err)
	}
}

func TestRun_MarkerSafety_ProceedsWithMarker(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "_site")

	// Create output dir WITH marker
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, ".repotool-output"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	// Minimal config
	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "templates"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "templates", "index.html.tmpl"),
		[]byte("<html>{{.RepoName}}</html>"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
	})
	if err != nil {
		t.Fatalf("Run should succeed with marker present: %v", err)
	}

	// Verify marker was recreated in new output
	if _, err := os.Stat(filepath.Join(output, ".repotool-output")); err != nil {
		t.Error("marker should be recreated in output")
	}
}

func TestRun_MarkerSafety_RefuseWhenOutputIsFile(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "_site")

	if err := os.WriteFile(output, []byte("not-a-dir"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
	})
	if err == nil {
		t.Fatal("expected error when output path is a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error should mention non-directory output path, got: %v", err)
	}
}

func TestRun_MarkerSafety_RefuseWhenOutputIsSymlink(t *testing.T) {
	root := t.TempDir()
	outputTarget := filepath.Join(root, "real-output")
	output := filepath.Join(root, "_site")

	if err := os.MkdirAll(outputTarget, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outputTarget, output); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   filepath.Join(root, "repo.toml"),
		TemplatePath: filepath.Join(root, "templates", "index.html.tmpl"),
	})
	if err == nil {
		t.Fatal("expected error when output path is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink output path, got: %v", err)
	}
}

func TestValidateOutputDir_RejectsSystemPaths(t *testing.T) {
	for _, path := range []string{"/", "/usr", "/home", "/etc", "/tmp", "/opt"} {
		if err := validateOutputDir(path); err == nil {
			t.Errorf("expected error for system path %q", path)
		}
	}
}

func TestValidateOutputDir_AllowsNormalPaths(t *testing.T) {
	if err := validateOutputDir("/var/lib/my-repo-output"); err != nil {
		t.Errorf("should allow normal path: %v", err)
	}
}
