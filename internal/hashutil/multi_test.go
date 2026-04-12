package hashutil

import (
	"strings"
	"testing"
)

func TestMultiHash(t *testing.T) {
	r := strings.NewReader("hello")
	sums, n, err := MultiHash(r)
	if err != nil {
		t.Fatalf("MultiHash returned error: %v", err)
	}
	if n != 5 {
		t.Fatalf("expected 5 bytes, got %d", n)
	}

	const wantSHA256 = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if sums.SHA256 != wantSHA256 {
		t.Errorf("SHA256 mismatch:\n  got  %s\n  want %s", sums.SHA256, wantSHA256)
	}

	if sums.MD5 == "" {
		t.Error("MD5 is empty")
	}
	if sums.SHA1 == "" {
		t.Error("SHA1 is empty")
	}
	if sums.SHA512 == "" {
		t.Error("SHA512 is empty")
	}
}
