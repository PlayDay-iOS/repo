package fileutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.txt")
	dstPath := filepath.Join(dir, "dst.txt")

	content := []byte("hello, copy test")

	// Source perm is intentionally unusual to verify CopyFile does not
	// inherit it: published artifacts are always written 0644.
	if err := os.WriteFile(srcPath, content, 0750); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFile: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}

	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("permission mismatch: got %v, want 0644", info.Mode().Perm())
	}
}

func TestCopyFileExclusive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.txt")
	dstPath := filepath.Join(dir, "dst.txt")

	content := []byte("exclusive copy test")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFileExclusive(srcPath, dstPath); err != nil {
		t.Fatalf("CopyFileExclusive: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("reading dst: %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("content mismatch: got %q, want %q", got, content)
	}
}

func TestCopyFileExclusive_FailsIfExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.txt")
	dstPath := filepath.Join(dir, "dst.txt")

	if err := os.WriteFile(srcPath, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dstPath, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}

	err := CopyFileExclusive(srcPath, dstPath)
	if err == nil {
		t.Fatal("expected error when dst exists, got nil")
	}
	if !os.IsExist(err) {
		t.Errorf("expected os.IsExist(err) to be true, got error: %v", err)
	}
}

