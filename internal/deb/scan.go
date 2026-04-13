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
		if d.IsDir() || !d.Type().IsRegular() || !strings.HasSuffix(strings.ToLower(d.Name()), ".deb") {
			return nil
		}

		entry, err := scanDebFile(path, d.Name(), cleanRoot, rootDir, allowedArchitectures)
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

// scanDebFile hashes, parses, and validates a single .deb file.
// The file handle is scoped to this function to keep the fd lifetime
// explicit: the ReaderAt passed to ExtractControlFromReader is only used
// within this frame, so the deferred Close cannot race with later use.
func scanDebFile(path, name, cleanRoot, rootDir string, allowedArchitectures map[string]bool) (*PackageEntry, error) {
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
	cleanRel := filepath.Clean(filepath.Join(cleanRoot, relPath))
	if !strings.HasPrefix(cleanRel, cleanRoot+string(os.PathSeparator)) {
		return nil, fmt.Errorf("file %s escapes root directory", path)
	}

	return &PackageEntry{
		Control:  control,
		Path:     path,
		Filename: filepath.ToSlash(relPath),
		Size:     size,
		MD5:      sums.MD5,
		SHA1:     sums.SHA1,
		SHA256:   sums.SHA256,
		SHA512:   sums.SHA512,
	}, nil
}
