package deb

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/hashutil"
)

// PackageEntry holds metadata for a single .deb found during pool scanning.
type PackageEntry struct {
	Control  *ControlData
	Path     string // absolute path to .deb file
	Filename string // relative path for Packages file
	Size     int64
	MD5      string
	SHA1     string
	SHA256   string
	SHA512   string
}

// generatedFields are fields appended by Stanza() and should be
// skipped if already present in the control data.
var generatedFields = map[string]bool{
	"filename": true, "size": true, "md5sum": true,
	"sha1": true, "sha256": true, "sha512": true,
}

// Stanza returns the RFC822-formatted Packages stanza for this entry.
func (e *PackageEntry) Stanza() string {
	var b strings.Builder
	for _, key := range e.Control.Order() {
		if generatedFields[strings.ToLower(key)] {
			continue
		}
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(e.Control.Get(key))
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "Filename: %s\n", e.Filename)
	fmt.Fprintf(&b, "Size: %d\n", e.Size)
	fmt.Fprintf(&b, "MD5sum: %s\n", e.MD5)
	fmt.Fprintf(&b, "SHA1: %s\n", e.SHA1)
	fmt.Fprintf(&b, "SHA256: %s\n", e.SHA256)
	fmt.Fprintf(&b, "SHA512: %s\n", e.SHA512)
	return b.String()
}

// ScanPool walks poolDir for .deb files. Filenames in returned entries
// are relative to rootDir (suitable for use in Packages files).
func ScanPool(rootDir, poolDir string, allowedArchitectures map[string]bool) ([]*PackageEntry, error) {
	var entries []*PackageEntry

	cleanRoot := filepath.Clean(rootDir)

	err := filepath.WalkDir(poolDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !d.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(d.Name()), ".deb") {
			return nil
		}

		// Stream hashes to avoid loading entire .deb into memory
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opening %s: %w", path, err)
		}
		defer f.Close()

		sums, size, err := hashutil.MultiHash(f)
		if err != nil {
			return fmt.Errorf("hashing %s: %w", path, err)
		}

		// Seek back to start for control extraction
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("seeking %s: %w", path, err)
		}

		control, err := ExtractControlFromReader(f, d.Name())
		if err != nil {
			return fmt.Errorf("extracting control from %s: %w", path, err)
		}

		if err := ValidateControl(control, allowedArchitectures); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		relPath, err := filepath.Rel(rootDir, path)
		if err != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, err)
		}
		cleanRel := filepath.Clean(filepath.Join(cleanRoot, relPath))
		if !strings.HasPrefix(cleanRel, cleanRoot+string(os.PathSeparator)) {
			return fmt.Errorf("file %s escapes root directory", path)
		}

		entries = append(entries, &PackageEntry{
			Control:  control,
			Path:     path,
			Filename: filepath.ToSlash(relPath),
			Size:     size,
			MD5:      sums.MD5,
			SHA1:     sums.SHA1,
			SHA256:   sums.SHA256,
			SHA512:   sums.SHA512,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Filename < entries[j].Filename
	})

	return entries, nil
}
