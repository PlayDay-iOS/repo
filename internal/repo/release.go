package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PlayDay-iOS/repo/internal/hashutil"
)

// ReleaseParams holds the metadata fields for a Release file.
type ReleaseParams struct {
	Origin        string
	Label         string
	Suite         string
	Codename      string
	Architectures string
	Components    string
	Description   string
	Date          time.Time
}

type fileHash struct {
	name   string
	size   int64
	md5    string
	sha1   string
	sha256 string
	sha512 string
}

// WriteRelease generates a Release file for the given directory.
// It hashes top-level index files only (Packages/Sources/Contents variants).
func WriteRelease(params ReleaseParams, dir string) error {
	hashes, err := hashDirectoryFiles(dir)
	if err != nil {
		return fmt.Errorf("hashing files in %s: %w", dir, err)
	}

	var b strings.Builder

	if params.Origin != "" {
		fmt.Fprintf(&b, "Origin: %s\n", params.Origin)
	}
	if params.Label != "" {
		fmt.Fprintf(&b, "Label: %s\n", params.Label)
	}
	fmt.Fprintf(&b, "Suite: %s\n", params.Suite)
	fmt.Fprintf(&b, "Codename: %s\n", params.Codename)
	fmt.Fprintf(&b, "Date: %s\n", params.Date.UTC().Format("Mon, 02 Jan 2006 15:04:05 UTC"))
	if params.Architectures != "" {
		fmt.Fprintf(&b, "Architectures: %s\n", params.Architectures)
	}
	if params.Components != "" {
		fmt.Fprintf(&b, "Components: %s\n", params.Components)
	}
	if params.Description != "" {
		fmt.Fprintf(&b, "Description: %s\n", params.Description)
	}

	if len(hashes) > 0 {
		b.WriteString("MD5Sum:\n")
		for _, h := range hashes {
			fmt.Fprintf(&b, " %s %16d %s\n", h.md5, h.size, h.name)
		}
		b.WriteString("SHA1:\n")
		for _, h := range hashes {
			fmt.Fprintf(&b, " %s %16d %s\n", h.sha1, h.size, h.name)
		}
		b.WriteString("SHA256:\n")
		for _, h := range hashes {
			fmt.Fprintf(&b, " %s %16d %s\n", h.sha256, h.size, h.name)
		}
		b.WriteString("SHA512:\n")
		for _, h := range hashes {
			fmt.Fprintf(&b, " %s %16d %s\n", h.sha512, h.size, h.name)
		}
	}

	return writeAtomic(filepath.Join(dir, "Release"), []byte(b.String()), 0644)
}

// writeAtomic writes data to path via a temp file + rename.
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// indexPrefixes lists the file-name prefixes that belong in a Release file.
// Only APT index files are hashed; non-index assets (icons, HTML pages) are excluded.
var indexPrefixes = []string{"Packages", "Sources", "Contents"}

func isIndexFile(name string) bool {
	for _, p := range indexPrefixes {
		if name == p || strings.HasPrefix(name, p+".") {
			return true
		}
	}
	return false
}

// hashDirectoryFiles hashes top-level APT index files only.
func hashDirectoryFiles(dir string) ([]fileHash, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var hashes []fileHash
	for _, de := range dirEntries {
		if de.IsDir() || !de.Type().IsRegular() || !isIndexFile(de.Name()) {
			continue
		}

		path := filepath.Join(dir, de.Name())
		h, err := hashFile(path, de.Name())
		if err != nil {
			return nil, err
		}
		hashes = append(hashes, h)
	}

	return hashes, nil
}

func hashFile(path, name string) (fileHash, error) {
	f, err := os.Open(path)
	if err != nil {
		return fileHash{}, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	sums, size, err := hashutil.MultiHash(f)
	if err != nil {
		return fileHash{}, fmt.Errorf("hashing %s: %w", path, err)
	}

	return fileHash{
		name:   name,
		size:   size,
		md5:    sums.MD5,
		sha1:   sums.SHA1,
		sha256: sums.SHA256,
		sha512: sums.SHA512,
	}, nil
}
