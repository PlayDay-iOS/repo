package repo

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/deb"
)

func Test_writePackages_Empty(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := writePackages(nil, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", buf.Len())
	}
}

func Test_writePackages_SingleEntry(t *testing.T) {
	t.Parallel()
	entries := []*deb.PackageEntry{{
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
	}}

	var buf bytes.Buffer
	if err := writePackages(entries, &buf); err != nil {
		t.Fatal(err)
	}

	out := buf.String()
	for _, want := range []string{
		"Package: com.test",
		"Filename: pool/stable/main/test.deb",
		"Size: 1234",
		"MD5sum: abc123",
		"SHA1: def456",
		"SHA256: 789ghi",
		"SHA512: jkl012",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("stanza missing %q", want)
		}
	}
}

func Test_writePackages_MultipleSeparated(t *testing.T) {
	t.Parallel()
	entries := []*deb.PackageEntry{
		{
			Control: deb.NewControlData(
				[]string{"Package"},
				map[string]string{"Package": "a"},
			),
			Filename: "a.deb", Size: 100, MD5: "m", SHA1: "s", SHA256: "h", SHA512: "k",
		},
		{
			Control: deb.NewControlData(
				[]string{"Package"},
				map[string]string{"Package": "b"},
			),
			Filename: "b.deb", Size: 200, MD5: "m", SHA1: "s", SHA256: "h", SHA512: "k",
		},
	}

	var buf bytes.Buffer
	if err := writePackages(entries, &buf); err != nil {
		t.Fatal(err)
	}

	if strings.Count(buf.String(), "\n\n") < 1 {
		t.Error("entries should be separated by blank line")
	}
}

func TestWritePackagesAll_CreatesAllFormats(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WritePackagesAll(context.Background(), nil, dir); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Packages", "Packages.gz", "Packages.xz", "Packages.bz2"} {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
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
