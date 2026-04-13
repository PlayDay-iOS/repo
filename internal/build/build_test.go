package build

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PlayDay-iOS/repo/internal/testutil"
)

// newTestRepo lays out a minimal repo skeleton and returns Options for Run.
// The provided debs (if any) are written into pool/<suite>/main.
func newTestRepo(t *testing.T, suite string, debs map[string][]byte) (string, Options) {
	t.Helper()
	root := t.TempDir()

	poolDir := filepath.Join(root, "pool", suite, "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, data := range debs {
		if err := os.WriteFile(filepath.Join(poolDir, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["`+suite+`"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "resources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources", "CydiaIcon.png"), []byte("PNG"), 0644); err != nil {
		t.Fatal(err)
	}

	return root, Options{
		RootDir:    root,
		OutputDir:  filepath.Join(root, "_site"),
		ConfigPath: filepath.Join(root, "repo.toml"),
		// TemplatePath is intentionally empty: build uses the embedded default.
	}
}

func TestRun_EmptyPool(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

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
		if _, err := os.Stat(filepath.Join(opts.OutputDir, f)); err != nil {
			t.Errorf("missing expected file: %s", f)
		}
	}
}

func TestRun_WithDeb(t *testing.T) {
	t.Parallel()
	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <t@t.com>"},
		{Key: "Description", Value: "Test package"},
	})

	_, opts := newTestRepo(t, "stable", map[string][]byte{"test.deb": debData})

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	pkgData, err := os.ReadFile(filepath.Join(opts.OutputDir, "stable", "Packages"))
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
	mirrorPath := filepath.Join(opts.OutputDir, "stable", "pool", "stable", "main", "test.deb")
	if _, err := os.Stat(mirrorPath); err != nil {
		t.Error("pool mirror should exist at stable/pool/stable/main/test.deb")
	}

	if _, err := os.Stat(filepath.Join(opts.OutputDir, "pool")); !os.IsNotExist(err) {
		t.Error("top-level pool/ should not exist in output")
	}
}

func TestRun_RejectsDisallowedArchitectureFromPool(t *testing.T) {
	t.Parallel()
	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "amd64"},
		{Key: "Maintainer", Value: "Test <t@t.com>"},
		{Key: "Description", Value: "Test package"},
	})

	_, opts := newTestRepo(t, "stable", map[string][]byte{"test.deb": debData})

	err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for disallowed architecture")
	}
	if !strings.Contains(err.Error(), "not allowed") {
		t.Errorf("error should mention disallowed architecture, got: %v", err)
	}
}

func TestRun_InvalidGPGKeyErrors(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)
	opts.GPGKey = "-----BEGIN PGP PRIVATE KEY BLOCK-----\nnotvalid\n-----END PGP PRIVATE KEY BLOCK-----"

	err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for invalid GPG key")
	}
	if !strings.Contains(err.Error(), "signing") && !strings.Contains(err.Error(), "GPG") {
		t.Errorf("error should mention signing/GPG, got: %v", err)
	}
}

func TestBuildTimeFromEnv_Unset(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "")
	before := time.Now().UTC().Add(-time.Second)
	got, err := BuildTimeFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC().Add(time.Second)
	if got.Before(before) || got.After(after) {
		t.Errorf("expected current time, got %v", got)
	}
}

func TestBuildTimeFromEnv_Valid(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	got, err := BuildTimeFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	want := time.Unix(1700000000, 0).UTC()
	if !got.Equal(want) {
		t.Errorf("expected %v, got %v", want, got)
	}
}

func TestBuildTimeFromEnv_Invalid(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")
	if _, err := BuildTimeFromEnv(); err == nil {
		t.Fatal("expected error for invalid SOURCE_DATE_EPOCH")
	}
}

func TestRun_MarkerSafety_RefuseWithoutMarker(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		t.Fatal(err)
	}

	err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error when output dir exists without marker")
	}
	if !strings.Contains(err.Error(), "marker") {
		t.Errorf("error should mention marker, got: %v", err)
	}
}

func TestRun_MarkerSafety_ProceedsWithMarker(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)

	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(opts.OutputDir, ".repotool-output"), nil, 0644); err != nil {
		t.Fatal(err)
	}

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run should succeed with marker present: %v", err)
	}

	if _, err := os.Stat(filepath.Join(opts.OutputDir, ".repotool-output")); err != nil {
		t.Error("marker should be recreated in output")
	}
}

func TestRun_MarkerSafety_RefuseWhenOutputIsFile(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)
	if err := os.WriteFile(opts.OutputDir, []byte("not-a-dir"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error when output path is a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error should mention non-directory output path, got: %v", err)
	}
}

func TestRun_MarkerSafety_RefuseWhenOutputIsSymlink(t *testing.T) {
	t.Parallel()
	root, opts := newTestRepo(t, "stable", nil)
	outputTarget := filepath.Join(root, "real-output")
	if err := os.MkdirAll(outputTarget, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outputTarget, opts.OutputDir); err != nil {
		t.Fatal(err)
	}

	err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error when output path is a symlink")
	}
	if !strings.Contains(err.Error(), "symlink") {
		t.Errorf("error should mention symlink output path, got: %v", err)
	}
}

func TestValidateOutputDir_RejectsSystemPaths(t *testing.T) {
	t.Parallel()
	for _, path := range []string{"/", "/usr", "/home", "/etc", "/tmp", "/opt"} {
		if err := validateOutputDir(path); err == nil {
			t.Errorf("expected error for system path %q", path)
		}
	}
}

func TestValidateOutputDir_AllowsNormalPaths(t *testing.T) {
	t.Parallel()
	if err := validateOutputDir("/var/lib/my-repo-output"); err != nil {
		t.Errorf("should allow normal path: %v", err)
	}
}
