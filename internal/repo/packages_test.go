package repo

import (
	"bytes"
	"strings"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/deb"
)

func Test_writePackages_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := writePackages(nil, &buf); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty output, got %d bytes", buf.Len())
	}
}

func Test_writePackages_SingleEntry(t *testing.T) {
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
	if !strings.Contains(out, "Package: com.test") {
		t.Error("missing Package field")
	}
	if !strings.Contains(out, "Filename: pool/stable/main/test.deb") {
		t.Error("missing Filename field")
	}
	if !strings.Contains(out, "Size: 1234") {
		t.Error("missing Size field")
	}
}

func Test_writePackages_MultipleSeparated(t *testing.T) {
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

	// Entries separated by blank line
	if strings.Count(buf.String(), "\n\n") < 1 {
		t.Error("entries should be separated by blank line")
	}
}
