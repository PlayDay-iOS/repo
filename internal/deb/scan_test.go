package deb

import (
	"context"
	"os"
	"path/filepath"
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
