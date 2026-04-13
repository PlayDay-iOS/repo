package hashutil

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"os"
)

// Sums holds hex-encoded hashes computed by MultiHash.
type Sums struct {
	MD5    string
	SHA1   string
	SHA256 string
	SHA512 string
}

// MultiHash computes MD5, SHA1, SHA256, and SHA512 in a single pass over r.
func MultiHash(r io.Reader) (Sums, int64, error) {
	md5h, sha1h, sha256h, sha512h := md5.New(), sha1.New(), sha256.New(), sha512.New()
	n, err := io.Copy(io.MultiWriter(md5h, sha1h, sha256h, sha512h), r)
	if err != nil {
		return Sums{}, n, err
	}
	return Sums{
		MD5:    hexEncode(md5h),
		SHA1:   hexEncode(sha1h),
		SHA256: hexEncode(sha256h),
		SHA512: hexEncode(sha512h),
	}, n, nil
}

// SHA256File returns the hex-encoded SHA256 of the file at path.
func SHA256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hexEncode(h hash.Hash) string {
	return hex.EncodeToString(h.Sum(nil))
}
