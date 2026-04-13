package ghimport

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

func TestReadAllowlist(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.txt")
	if err := os.WriteFile(path, []byte("# comment\n\nrepo-one\nrepo-two\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repos, err := ReadAllowlist(slog.Default(), path)
	if err != nil {
		t.Fatalf("ReadAllowlist failed: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(repos))
	}
	if repos[0] != "repo-one" || repos[1] != "repo-two" {
		t.Errorf("unexpected repos: %v", repos)
	}
}

func TestReadAllowlist_Empty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "allowlist.txt")
	if err := os.WriteFile(path, []byte("# only comments\n\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repos, err := ReadAllowlist(slog.Default(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(repos) != 0 {
		t.Errorf("expected empty, got %v", repos)
	}
}

func TestReadAllowlist_Missing(t *testing.T) {
	t.Parallel()
	_, err := ReadAllowlist(slog.Default(), "/nonexistent/path")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
