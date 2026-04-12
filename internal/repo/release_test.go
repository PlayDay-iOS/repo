package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteRelease_BasicFields(t *testing.T) {
	dir := t.TempDir()

	// Write a dummy Packages file so there's something to hash
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

	if err := WriteRelease(params, dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Release"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	checks := []string{
		"Origin: TestOrigin",
		"Label: TestLabel",
		"Suite: stable",
		"Codename: stable",
		"MD5Sum:",
		"SHA256:",
		"Packages",
	}
	for _, check := range checks {
		if !strings.Contains(content, check) {
			t.Errorf("Release missing %q", check)
		}
	}
}

func TestWriteRelease_SkipsSelfReference(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Packages"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	// Pre-existing Release should be skipped during hashing
	if err := os.WriteFile(filepath.Join(dir, "Release"), []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	params := ReleaseParams{Suite: "stable", Codename: "stable", Date: time.Now()}
	if err := WriteRelease(params, dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "Release"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	// Should only list "Packages", not "Release"
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, " Release") {
			t.Error("Release should not hash itself")
		}
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
