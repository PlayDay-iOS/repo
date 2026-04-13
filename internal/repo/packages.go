package repo

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"path/filepath"

	"github.com/PlayDay-iOS/repo/internal/deb"
	"github.com/PlayDay-iOS/repo/internal/fileutil"
	"github.com/dsnet/compress/bzip2"
	"github.com/ulikunitz/xz"
)

// writePackages writes the Packages index (plain text) from the given entries.
func writePackages(entries []*deb.PackageEntry, w io.Writer) error {
	for i, e := range entries {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(w, e.Stanza()); err != nil {
			return err
		}
	}
	return nil
}

// WritePackagesAll writes Packages, Packages.gz, Packages.xz, and Packages.bz2
// into the target directory atomically.
func WritePackagesAll(ctx context.Context, entries []*deb.PackageEntry, dir string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := writePackages(entries, &buf); err != nil {
		return fmt.Errorf("generating packages: %w", err)
	}
	raw := buf.Bytes()

	if err := fileutil.WriteAtomicBytes(filepath.Join(dir, "Packages"), 0644, raw); err != nil {
		return fmt.Errorf("writing Packages: %w", err)
	}

	compressors := []struct {
		ext string
		fn  compressFunc
	}{
		{".gz", compressGzip},
		{".xz", compressXZ},
		{".bz2", compressBZ2},
	}
	for _, c := range compressors {
		path := filepath.Join(dir, "Packages"+c.ext)
		if err := writeCompressed(path, raw, c.fn); err != nil {
			return fmt.Errorf("writing %s: %w", filepath.Base(path), err)
		}
	}

	return nil
}

type compressFunc func(w io.Writer) (io.WriteCloser, error)

func compressGzip(w io.Writer) (io.WriteCloser, error) {
	return gzip.NewWriter(w), nil
}

func compressXZ(w io.Writer) (io.WriteCloser, error) {
	return xz.NewWriter(w)
}

func compressBZ2(w io.Writer) (io.WriteCloser, error) {
	return bzip2.NewWriter(w, nil)
}

func writeCompressed(path string, data []byte, newWriter compressFunc) error {
	return fileutil.WriteAtomic(path, 0644, func(w io.Writer) error {
		cw, err := newWriter(w)
		if err != nil {
			return err
		}
		if _, err := cw.Write(data); err != nil {
			cw.Close()
			return err
		}
		return cw.Close()
	})
}
