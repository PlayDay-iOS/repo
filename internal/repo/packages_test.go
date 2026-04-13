package repo

import (
	"compress/gzip"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/deb"
)

func readPackagesGz(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	data, err := io.ReadAll(gz)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestWritePackagesAll_EmptyEntries(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WritePackagesAll(context.Background(), nil, dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Packages", "Packages.gz", "Packages.xz", "Packages.bz2"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		if name == "Packages" && info.Size() != 0 {
			t.Errorf("Packages with no entries should be empty, got %d bytes", info.Size())
		}
	}
}

func TestWritePackagesAll_RendersStanzas(t *testing.T) {
	t.Parallel()
	entries := []*deb.PackageEntry{
		{
			Control: deb.NewControlData(
				[]string{"Package", "Version"},
				map[string]string{"Package": "com.test", "Version": "1.0"},
			),
			Filename: "pool/stable/main/test.deb",
			Size:     1234,
			MD5:      "abc123",
			SHA1:     "def456",
			SHA256:   "789ghi",
			SHA512:   "jkl012",
		},
		{
			Control: deb.NewControlData(
				[]string{"Package"},
				map[string]string{"Package": "com.other"},
			),
			Filename: "pool/stable/main/other.deb",
			Size:     200,
			MD5:      "m", SHA1: "s", SHA256: "h", SHA512: "k",
		},
	}

	dir := t.TempDir()
	if err := WritePackagesAll(context.Background(), entries, dir); err != nil {
		t.Fatal(err)
	}

	plain, err := os.ReadFile(filepath.Join(dir, "Packages"))
	if err != nil {
		t.Fatal(err)
	}
	out := string(plain)
	for _, want := range []string{
		"Package: com.test",
		"Filename: pool/stable/main/test.deb",
		"Size: 1234",
		"MD5sum: abc123",
		"SHA1: def456",
		"SHA256: 789ghi",
		"SHA512: jkl012",
		"Package: com.other",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("Packages missing %q", want)
		}
	}
	if strings.Count(out, "\n\n") < 1 {
		t.Error("multiple entries should be separated by blank line")
	}

	if got := readPackagesGz(t, filepath.Join(dir, "Packages.gz")); got != out {
		t.Errorf("Packages.gz content differs from Packages")
	}
}

func TestWritePackagesAll_CancelledContext(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := WritePackagesAll(ctx, nil, t.TempDir()); err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
