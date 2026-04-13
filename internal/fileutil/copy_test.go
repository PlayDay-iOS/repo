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
	perm := os.FileMode(0750)

	if err := os.WriteFile(srcPath, content, perm); err != nil {
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
	if info.Mode().Perm() != perm {
		t.Errorf("permission mismatch: got %v, want %v", info.Mode().Perm(), perm)
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

func TestCopyDir(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()

	// Create nested structure: src/a.txt, src/sub/b.txt
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("aaa"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("bbb"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}

	for _, rel := range []string{"a.txt", filepath.Join("sub", "b.txt")} {
		srcData, _ := os.ReadFile(filepath.Join(src, rel))
		dstData, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("missing %s in dst: %v", rel, err)
			continue
		}
		if string(dstData) != string(srcData) {
			t.Errorf("%s: got %q, want %q", rel, dstData, srcData)
		}
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

func TestCopyDir_SkipsHiddenEntries(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()

	if err := os.WriteFile(filepath.Join(src, "real.deb"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".gitkeep"), nil, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(src, ".cache", "junk"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, ".cache", "junk", "x"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyDir(src, dst); err != nil {
		t.Fatalf("CopyDir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "real.deb")); err != nil {
		t.Errorf("real.deb missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, ".gitkeep")); !os.IsNotExist(err) {
		t.Error(".gitkeep should be skipped")
	}
	if _, err := os.Stat(filepath.Join(dst, ".cache")); !os.IsNotExist(err) {
		t.Error(".cache directory should be skipped")
	}
}

func TestCopyDir_RejectsSymlink(t *testing.T) {
	t.Parallel()

	src := t.TempDir()
	dst := t.TempDir()
	target := t.TempDir()

	if err := os.Symlink(target, filepath.Join(src, "link")); err != nil {
		t.Fatal(err)
	}

	err := CopyDir(src, dst)
	if err == nil {
		t.Fatal("expected error for symlink in source tree")
	}
}
