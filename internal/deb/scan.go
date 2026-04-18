package deb

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/hashutil"
)

// PackageEntry holds metadata for a single .deb found during pool scanning.
type PackageEntry struct {
	Control       *ControlData
	Path          string // absolute path to .deb file (may be a symlink)
	CanonicalPath string // resolved absolute path (after EvalSymlinks)
	Filename      string // relative path for Packages file
	Size          int64
	MD5           string
	SHA1          string
	SHA256        string
	SHA512        string
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
	writeField := func(key, val string) {
		b.WriteString(key)
		b.WriteString(": ")
		b.WriteString(val)
		b.WriteByte('\n')
	}
	for _, key := range e.Control.Order() {
		if generatedFields[strings.ToLower(key)] {
			continue
		}
		writeField(key, e.Control.Get(key))
	}
	writeField("Filename", e.Filename)
	writeField("Size", strconv.FormatInt(e.Size, 10))
	writeField("MD5sum", e.MD5)
	writeField("SHA1", e.SHA1)
	writeField("SHA256", e.SHA256)
	writeField("SHA512", e.SHA512)
	return b.String()
}

// ScanPool walks poolDir for .deb files. Filenames in returned entries
// are relative to rootDir (suitable for use in Packages files).
func ScanPool(ctx context.Context, rootDir, poolDir string, allowedArchitectures map[string]bool) ([]*PackageEntry, error) {
	var entries []*PackageEntry

	cleanRoot := filepath.Clean(rootDir)

	err := filepath.WalkDir(poolDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".deb") {
			return nil
		}
		isRegular := d.Type().IsRegular()
		if d.Type()&fs.ModeSymlink != 0 {
			fi, statErr := os.Stat(path)
			if statErr != nil {
				return fmt.Errorf("following symlink %s: %w", path, statErr)
			}
			isRegular = fi.Mode().IsRegular()
		}
		if !isRegular {
			return nil
		}

		entry, err := scanDebFile(path, d.Name(), cleanRoot, allowedArchitectures)
		if err != nil {
			return err
		}
		entries = append(entries, entry)
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

// CanonicalSuite extracts the suite name from a canonical pool path.
// canonicalPath must be an absolute path under rootDir following the
// pool/<suite>/<component>/<file> layout.
func CanonicalSuite(rootDir, canonicalPath string) (string, error) {
	rel, err := filepath.Rel(rootDir, canonicalPath)
	if err != nil {
		return "", fmt.Errorf("computing relative path: %w", err)
	}
	parts := strings.SplitN(filepath.ToSlash(rel), "/", 4)
	if len(parts) < 3 || parts[0] != "pool" {
		return "", fmt.Errorf("path %s not in expected pool/<suite>/<component>/ layout", canonicalPath)
	}
	return parts[1], nil
}

// scanDebFile hashes, parses, and validates a single .deb file.
// rootDir must already be cleaned by the caller.
func scanDebFile(path, name, rootDir string, allowedArchitectures map[string]bool) (*PackageEntry, error) {
	canonical, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("resolving symlink %s: %w", path, err)
	}
	canonical = filepath.Clean(canonical)
	if !strings.HasPrefix(canonical, rootDir+string(os.PathSeparator)) {
		return nil, fmt.Errorf("file %s resolves to %s outside root directory", path, canonical)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	sums, size, err := hashutil.MultiHash(f)
	if err != nil {
		return nil, fmt.Errorf("hashing %s: %w", path, err)
	}

	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("seeking %s: %w", path, err)
	}

	control, err := ExtractControlFromReader(f, name)
	if err != nil {
		return nil, fmt.Errorf("extracting control from %s: %w", path, err)
	}

	if err := ValidateControl(control, allowedArchitectures); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}

	relPath, err := filepath.Rel(rootDir, path)
	if err != nil {
		return nil, fmt.Errorf("computing relative path for %s: %w", path, err)
	}
	cleanRel := filepath.Clean(filepath.Join(rootDir, relPath))
	if !strings.HasPrefix(cleanRel, rootDir+string(os.PathSeparator)) {
		return nil, fmt.Errorf("file %s escapes root directory", path)
	}

	return &PackageEntry{
		Control:       control,
		Path:          path,
		CanonicalPath: canonical,
		Filename:      filepath.ToSlash(relPath),
		Size:          size,
		MD5:           sums.MD5,
		SHA1:          sums.SHA1,
		SHA256:        sums.SHA256,
		SHA512:        sums.SHA512,
	}, nil
}
