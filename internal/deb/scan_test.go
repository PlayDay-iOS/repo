package deb

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/testutil"
)

func TestScanPool_FindsDebs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	poolDir := filepath.Join(dir, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <test@test.com>"},
		{Key: "Description", Value: "Test"},
	})
	if err := os.WriteFile(filepath.Join(poolDir, "test.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true})
	if err != nil {
		t.Fatalf("ScanPool failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Control.Get("Package") != "com.test.pkg" {
		t.Errorf("Package = %q", entries[0].Control.Get("Package"))
	}
	if entries[0].Filename != "pool/stable/main/test.deb" {
		t.Errorf("Filename = %q", entries[0].Filename)
	}
	if entries[0].MD5 == "" || entries[0].SHA1 == "" || entries[0].SHA256 == "" || entries[0].SHA512 == "" {
		t.Error("all hash fields should be populated")
	}
}

func TestScanPool_EmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	poolDir := filepath.Join(dir, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}

	entries, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestScanPool_IgnoresNonDeb(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	poolDir := filepath.Join(dir, "pool")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(poolDir, "readme.txt"), []byte("not a deb"), 0644); err != nil {
		t.Fatal(err)
	}

	entries, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestScanPool_RejectsDisallowedArchitecture(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	poolDir := filepath.Join(dir, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "amd64"},
		{Key: "Maintainer", Value: "Test <test@test.com>"},
		{Key: "Description", Value: "Test"},
	})
	if err := os.WriteFile(filepath.Join(poolDir, "test.deb"), debData, 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true}); err == nil {
		t.Fatal("expected error for disallowed architecture")
	}
}

func TestScanPool_FollowsSymlinkToDebInSameSuite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	poolDir := filepath.Join(dir, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <test@test.com>"},
		{Key: "Description", Value: "Test"},
	})
	realPath := filepath.Join(poolDir, "real.deb")
	if err := os.WriteFile(realPath, debData, 0644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(poolDir, "link.deb")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	entries, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true})
	if err != nil {
		t.Fatalf("ScanPool failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Both entries should have CanonicalPath pointing to real.deb
	for _, e := range entries {
		if e.CanonicalPath == "" {
			t.Error("CanonicalPath should be set")
		}
		if !strings.HasSuffix(e.CanonicalPath, "real.deb") && !strings.HasSuffix(e.Path, "real.deb") {
			// link.deb's CanonicalPath should resolve to real.deb
			if strings.HasSuffix(e.Path, "link.deb") && !strings.HasSuffix(e.CanonicalPath, "real.deb") {
				t.Errorf("link.deb CanonicalPath = %q, want suffix real.deb", e.CanonicalPath)
			}
		}
	}
}

func TestScanPool_FollowsSymlinkToDebInOtherSuite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	stableDir := filepath.Join(dir, "pool", "stable", "main")
	betaDir := filepath.Join(dir, "pool", "beta", "main")
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
	realPath := filepath.Join(stableDir, "test.deb")
	if err := os.WriteFile(realPath, debData, 0644); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(betaDir, "test.deb")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	entries, err := ScanPool(context.Background(), dir, betaDir, map[string]bool{"iphoneos-arm64": true})
	if err != nil {
		t.Fatalf("ScanPool failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Filename != "pool/beta/main/test.deb" {
		t.Errorf("Filename = %q, want pool/beta/main/test.deb", e.Filename)
	}
	if !strings.HasSuffix(e.CanonicalPath, filepath.Join("pool", "stable", "main", "test.deb")) {
		t.Errorf("CanonicalPath = %q, want suffix pool/stable/main/test.deb", e.CanonicalPath)
	}
}

func TestScanPool_BrokenSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	poolDir := filepath.Join(dir, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(poolDir, "nonexistent.deb"), filepath.Join(poolDir, "broken.deb")); err != nil {
		t.Fatal(err)
	}

	_, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true})
	if err == nil {
		t.Fatal("expected error for broken symlink")
	}
}

func TestScanPool_SymlinkCycle(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	poolDir := filepath.Join(dir, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create a→b and b→a cycle
	a := filepath.Join(poolDir, "a.deb")
	b := filepath.Join(poolDir, "b.deb")
	if err := os.Symlink(b, a); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(a, b); err != nil {
		t.Fatal(err)
	}

	_, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true})
	if err == nil {
		t.Fatal("expected error for symlink cycle")
	}
}

func TestScanPool_SymlinkOutsideRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outsideDir := t.TempDir()
	poolDir := filepath.Join(dir, "pool", "stable", "main")
	if err := os.MkdirAll(poolDir, 0755); err != nil {
		t.Fatal(err)
	}

	debData := testutil.BuildMinimalDeb([]testutil.Field{
		{Key: "Package", Value: "com.test.pkg"},
		{Key: "Version", Value: "1.0"},
		{Key: "Architecture", Value: "iphoneos-arm64"},
		{Key: "Maintainer", Value: "Test <test@test.com>"},
		{Key: "Description", Value: "Test"},
	})
	outsidePath := filepath.Join(outsideDir, "outside.deb")
	if err := os.WriteFile(outsidePath, debData, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsidePath, filepath.Join(poolDir, "escape.deb")); err != nil {
		t.Fatal(err)
	}

	_, err := ScanPool(context.Background(), dir, poolDir, map[string]bool{"iphoneos-arm64": true})
	if err == nil {
		t.Fatal("expected error for symlink outside root")
	}
	if !strings.Contains(err.Error(), "outside root") {
		t.Errorf("error should mention 'outside root', got: %v", err)
	}
}
