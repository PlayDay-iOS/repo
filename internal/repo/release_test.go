package repo

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteRelease_BasicFields(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	if err := os.WriteFile(filepath.Join(dir, "Packages"), []byte("Package: test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	params := ReleaseParams{
		Origin:        "TestOrigin",
		Label:         "TestLabel",
		Suite:         "stable",
		Codename:      "stable",
		Architectures: "iphoneos-arm64 all",
		Description:   "Test description",
		Date:          time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	if err := WriteRelease(context.Background(), params, dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Release"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	for _, check := range []string{
		"Origin: TestOrigin",
		"Label: TestLabel",
		"Suite: stable",
		"Codename: stable",
		"MD5Sum:",
		"SHA1:",
		"SHA256:",
		"SHA512:",
		"Packages",
	} {
		if !strings.Contains(content, check) {
			t.Errorf("Release missing %q", check)
		}
	}
}

func TestWriteRelease_ExcludesNonIndexFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Packages"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	// Pre-existing Release should be skipped during hashing
	if err := os.WriteFile(filepath.Join(dir, "Release"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	params := ReleaseParams{Suite: "stable", Codename: "stable", Date: time.Now()}
	if err := WriteRelease(context.Background(), params, dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Release"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasSuffix(strings.TrimSpace(line), " Release") {
			t.Error("Release should not hash itself")
		}
	}
}

func TestWriteRelease_CancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := WriteRelease(ctx, ReleaseParams{Suite: "stable", Codename: "stable"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestIsIndexFile(t *testing.T) {
	t.Parallel()

	shouldMatch := []string{
		"Packages", "Packages.gz", "Packages.xz", "Packages.bz2",
		"Sources", "Sources.gz", "Sources.xz",
		"Contents", "Contents.gz",
	}
	for _, name := range shouldMatch {
		if !isIndexFile(name) {
			t.Errorf("isIndexFile(%q) = false, want true", name)
		}
	}

	shouldReject := []string{
		"Release", "Release.gpg", "InRelease",
		"CydiaIcon.png", "index.html", ".repotool-output",
	}
	for _, name := range shouldReject {
		if isIndexFile(name) {
			t.Errorf("isIndexFile(%q) = true, want false", name)
		}
	}
}
