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
[github]
org_name = "TestOrg"
[hosting]
repo = "testrepo"
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "resources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources", "CydiaIcon.png"), []byte("PNG"), 0644); err != nil {
		t.Fatal(err)
	}

	writeStubDeb(t, root)

	return root, Options{
		RootDir:    root,
		OutputDir:  filepath.Join(root, "_site"),
		IndexDir:   filepath.Join(root, "_index"),
		ConfigPath: filepath.Join(root, "repo.toml"),
		// TemplatePath is intentionally empty: build uses the embedded default.
	}
}

// writeStubDeb installs the migration stub that the legacy Pages index
// advertises in place of the real packages.
func writeStubDeb(t *testing.T, root string) string {
	t.Helper()
	dir := filepath.Join(root, "resources", "source-moved")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	name := "dev.playday3008.source-moved_1.0.0_all.deb"
	data := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "dev.playday3008.source-moved"},
		{Key: "Name", Value: "Repository Moved"},
		{Key: "Version", Value: "1.0.0"},
		{Key: "Architecture", Value: "all"},
		{Key: "Maintainer", Value: "PlayDay-iOS"},
		{Key: "Description", Value: "placeholder"},
	})
	if err := os.WriteFile(filepath.Join(dir, name), data, 0644); err != nil {
		t.Fatal(err)
	}
	return name
}

func TestRun_EmptyPool(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	for _, f := range []string{
		"stable/Packages",
		"stable/Packages.gz",
		"stable/Packages.xz",
		"stable/Packages.bz2",
		"stable/Release",
		"stable/CydiaIcon.png",
	} {
		if _, err := os.Stat(filepath.Join(opts.IndexDir, f)); err != nil {
			t.Errorf("missing expected index file: %s", f)
		}
	}

	for _, f := range []string{
		"index.html",
		"CydiaIcon.png",
		"stable/Packages",
		"stable/Release",
		"stable/CydiaIcon.png",
		"stable/index.html",
	} {
		if _, err := os.Stat(filepath.Join(opts.OutputDir, f)); err != nil {
			t.Errorf("missing expected site file: %s", f)
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

	pkgData, err := os.ReadFile(filepath.Join(opts.IndexDir, "stable", "Packages"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(pkgData)
	if !strings.Contains(content, "Package: com.test.pkg") {
		t.Error("Packages should contain the package")
	}
	if !strings.Contains(content, "Filename: ./test.deb") {
		t.Errorf("Packages should contain a Filename relative to the source base:\n%s", content)
	}
	if !strings.Contains(content, "Depiction: https://example.com/repo/depictions/test/depiction.html") {
		t.Errorf("Packages should contain injected Depiction URL:\n%s", content)
	}
	if !strings.Contains(content, "SileoDepiction: https://example.com/repo/depictions/test/sileo.json") {
		t.Errorf("Packages should contain injected SileoDepiction URL:\n%s", content)
	}

	// No .deb mirror in output — payloads are on GitHub Releases
	mirrorPath := filepath.Join(opts.IndexDir, "stable", "pool")
	if _, err := os.Stat(mirrorPath); !os.IsNotExist(err) {
		t.Error("pool mirror should NOT exist in output with releases hosting")
	}

	htmlPath := filepath.Join(opts.OutputDir, "depictions", "test", "depiction.html")
	if _, err := os.Stat(htmlPath); err != nil {
		t.Errorf("missing depiction.html: %v", err)
	}
	jsonPath := filepath.Join(opts.OutputDir, "depictions", "test", "sileo.json")
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("missing sileo.json: %v", err)
	}
	cssPath := filepath.Join(opts.OutputDir, "depictions", "style.css")
	if _, err := os.Stat(cssPath); err != nil {
		t.Errorf("missing depictions/style.css: %v", err)
	}
}

func TestRun_EmptyPoolSkipsDepictionsDir(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(opts.OutputDir, "depictions")); !os.IsNotExist(err) {
		t.Error("depictions/ should not exist when no entries")
	}
}

func TestRun_Reproducible(t *testing.T) {
	t.Parallel()
	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <t@t.com>"},
		{Key: "Description", Value: "Test package"},
	})

	_, opts := newTestRepo(t, "stable", map[string][]byte{"test.deb": debData})
	opts.BuildTime = time.Unix(1700000000, 0).UTC()

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("first Run failed: %v", err)
	}
	pkgFirst, _ := os.ReadFile(filepath.Join(opts.IndexDir, "stable", "Packages"))
	htmlFirst, _ := os.ReadFile(filepath.Join(opts.OutputDir, "depictions", "test", "depiction.html"))
	jsonFirst, _ := os.ReadFile(filepath.Join(opts.OutputDir, "depictions", "test", "sileo.json"))

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("second Run failed: %v", err)
	}
	pkgSecond, _ := os.ReadFile(filepath.Join(opts.IndexDir, "stable", "Packages"))
	htmlSecond, _ := os.ReadFile(filepath.Join(opts.OutputDir, "depictions", "test", "depiction.html"))
	jsonSecond, _ := os.ReadFile(filepath.Join(opts.OutputDir, "depictions", "test", "sileo.json"))

	if string(pkgFirst) != string(pkgSecond) {
		t.Error("Packages output drifted between runs")
	}
	if string(htmlFirst) != string(htmlSecond) {
		t.Error("depiction.html drifted between runs")
	}
	if string(jsonFirst) != string(jsonSecond) {
		t.Error("sileo.json drifted between runs")
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

func TestRun_MissingDepictionTemplatePath_HardFailsAtStart(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)
	opts.DepictionTemplatePath = filepath.Join(t.TempDir(), "does-not-exist.tmpl")

	err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for missing depiction template path")
	}
	if !strings.Contains(err.Error(), "--depiction-template") {
		t.Errorf("error should name the flag, got: %v", err)
	}
	// Build should have stopped before writing the output marker.
	if _, statErr := os.Stat(filepath.Join(opts.OutputDir, ".repotool-output")); !os.IsNotExist(statErr) {
		t.Error("output dir should not have been touched before validation failure")
	}
}

func TestRun_MissingDepictionStylePath_HardFailsAtStart(t *testing.T) {
	t.Parallel()
	_, opts := newTestRepo(t, "stable", nil)
	opts.DepictionStylePath = filepath.Join(t.TempDir(), "does-not-exist.css")

	err := Run(context.Background(), opts)
	if err == nil {
		t.Fatal("expected error for missing depiction style path")
	}
	if !strings.Contains(err.Error(), "--depiction-style") {
		t.Errorf("error should name the flag, got: %v", err)
	}
}

func newTestRepoMultiSuite(t *testing.T, stableDebs map[string][]byte, betaSymlinks []string) (string, Options) {
	t.Helper()
	root := t.TempDir()

	stablePool := filepath.Join(root, "pool", "stable", "main")
	betaPool := filepath.Join(root, "pool", "beta", "main")
	if err := os.MkdirAll(stablePool, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(betaPool, 0755); err != nil {
		t.Fatal(err)
	}

	for name, data := range stableDebs {
		if err := os.WriteFile(filepath.Join(stablePool, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range betaSymlinks {
		target := filepath.Join(stablePool, name)
		link := filepath.Join(betaPool, name)
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(root, "repo.toml"), []byte(`
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["stable", "beta"]
[github]
org_name = "TestOrg"
[hosting]
repo = "testrepo"
`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(root, "resources"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "resources", "CydiaIcon.png"), []byte("PNG"), 0644); err != nil {
		t.Fatal(err)
	}

	writeStubDeb(t, root)

	return root, Options{
		RootDir:    root,
		OutputDir:  filepath.Join(root, "_site"),
		IndexDir:   filepath.Join(root, "_index"),
		ConfigPath: filepath.Join(root, "repo.toml"),
	}
}

func TestRun_LegacySiteIndexAdvertisesOnlyTheStub(t *testing.T) {
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

	data, err := os.ReadFile(filepath.Join(opts.OutputDir, "stable", "Packages"))
	if err != nil {
		t.Fatal(err)
	}
	site := string(data)

	// The old Pages URL cannot serve payloads, so advertising real packages
	// there would reproduce the 404 this whole change exists to remove.
	if strings.Contains(site, "Package: com.test.pkg") {
		t.Errorf("superseded Pages index must not advertise real packages:\n%s", site)
	}
	if !strings.Contains(site, "Package: dev.playday3008.source-moved") {
		t.Errorf("superseded Pages index should advertise the stub:\n%s", site)
	}
	if !strings.Contains(site, "Filename: ./dev.playday3008.source-moved_1.0.0_all.deb") {
		t.Errorf("stub Filename should be relative to the Pages suite dir:\n%s", site)
	}
	if !strings.Contains(site, "https://github.com/TestOrg/testrepo/releases/download/pool-stable/") {
		t.Errorf("stub should name the new source URL:\n%s", site)
	}

	stubPath := filepath.Join(opts.OutputDir, "stable", "dev.playday3008.source-moved_1.0.0_all.deb")
	if _, err := os.Stat(stubPath); err != nil {
		t.Errorf("stub payload should be served next to the index it is listed in: %v", err)
	}
}

func TestRun_SymlinkCrossSuite_UsesInSuiteRelativeFilename(t *testing.T) {
	t.Parallel()
	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <t@t.com>"},
		{Key: "Description", Value: "Test package"},
	})

	_, opts := newTestRepoMultiSuite(t, map[string][]byte{"test.deb": debData}, []string{"test.deb"})

	if err := Run(context.Background(), opts); err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// Stable's own copy is served from the stable release.
	stableData, err := os.ReadFile(filepath.Join(opts.IndexDir, "stable", "Packages"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(stableData), "Filename: ./test.deb") {
		t.Errorf("stable Packages should reference the deb relative to the stable source base:\n%s", string(stableData))
	}

	betaData, err := os.ReadFile(filepath.Join(opts.IndexDir, "beta", "Packages"))
	if err != nil {
		t.Fatal(err)
	}
	// A symlink resolving into stable is still served from beta's own release,
	// so its Filename must stay relative to the beta source base.
	if !strings.Contains(string(betaData), "Filename: ./test.deb") {
		t.Errorf("beta Packages should reference the deb relative to the beta source base:\n%s", string(betaData))
	}
	if strings.Contains(string(betaData), "pool-stable") {
		t.Errorf("beta Packages must not reference another suite's release:\n%s", string(betaData))
	}

	// No .deb mirror in either suite
	for _, suite := range []string{"stable", "beta"} {
		poolDir := filepath.Join(opts.IndexDir, suite, "pool")
		if _, err := os.Stat(poolDir); !os.IsNotExist(err) {
			t.Errorf("%s should have no pool/ mirror in output", suite)
		}
	}
}
