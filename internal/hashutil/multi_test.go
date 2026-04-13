package hashutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMultiHash(t *testing.T) {
	t.Parallel()
	r := strings.NewReader("hello")
	sums, n, err := MultiHash(r)
	if err != nil {
		t.Fatalf("MultiHash returned error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}

	expected := map[string]string{
		"MD5":    "5d41402abc4b2a76b9719d911017c592",
		"SHA1":   "aaf4c61ddcc5e8a2dabede0f3b482cd9aea9434d",
		"SHA256": "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824",
		"SHA512": "9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043",
	}
	got := map[string]string{
		"MD5": sums.MD5, "SHA1": sums.SHA1, "SHA256": sums.SHA256, "SHA512": sums.SHA512,
	}
	for name, want := range expected {
		if got[name] != want {
			t.Errorf("%s mismatch:\n  got  %s\n  want %s", name, got[name], want)
		}
	}
}

func TestSHA256File(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "f.bin")
	if err := os.WriteFile(path, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	got, err := SHA256File(path)
	if err != nil {
		t.Fatalf("SHA256File: %v", err)
	}
	const want = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestSHA256File_Missing(t *testing.T) {
	t.Parallel()
	if _, err := SHA256File("/nonexistent/file"); err == nil {
		t.Fatal("expected error for missing file")
	}
}
