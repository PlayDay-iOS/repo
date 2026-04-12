package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestLoadGPGKey_EnvTakesPrecedence(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.asc")
	if err := os.WriteFile(keyFile, []byte("file-key"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GPG_PRIVATE_KEY", "env-key")
	got, err := loadGPGKey(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "env-key" {
		t.Errorf("expected env key, got %q", got)
	}
}

func TestLoadGPGKey_ReadsFile(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "key.asc")
	if err := os.WriteFile(keyFile, []byte("file-key"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GPG_PRIVATE_KEY", "")
	got, err := loadGPGKey(keyFile)
	if err != nil {
		t.Fatal(err)
	}
	if got != "file-key" {
		t.Errorf("expected file key, got %q", got)
	}
}

func TestLoadGPGKey_EmptyReturnsEmpty(t *testing.T) {
	t.Setenv("GPG_PRIVATE_KEY", "")
	got, err := loadGPGKey("")
	if err != nil {
		t.Fatal(err)
	}
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestLoadGPGKey_MissingFileErrors(t *testing.T) {
	t.Setenv("GPG_PRIVATE_KEY", "")
	_, err := loadGPGKey("/nonexistent/key.asc")
	if err == nil {
		t.Fatal("expected error for missing file")
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
	for _, path := range []string{"/", "/usr", "/home", "/etc"} {
		if err := validateOutputDir(path); err == nil {
			t.Errorf("expected error for system path %q", path)
		}
	}
}

func TestValidateOutputDir_AllowsNormalPaths(t *testing.T) {
	if err := validateOutputDir("/tmp/my-repo-output"); err != nil {
		t.Errorf("should allow normal path: %v", err)
	}
}
