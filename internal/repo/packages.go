package repo

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/PlayDay-iOS/repo/internal/deb"
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
// into the target directory.
func WritePackagesAll(entries []*deb.PackageEntry, dir string) error {
	var buf bytes.Buffer
	if err := writePackages(entries, &buf); err != nil {
		return fmt.Errorf("generating packages: %w", err)
	}
	raw := buf.Bytes()

	if err := os.WriteFile(filepath.Join(dir, "Packages"), raw, 0644); err != nil {
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
		if err := writeCompressed(filepath.Join(dir, "Packages"+c.ext), raw, c.fn); err != nil {
			return err
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

func writeCompressed(path string, data []byte, newWriter compressFunc) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			f.Close()
		}
		if err != nil {
			os.Remove(path)
		}
	}()

	cw, err := newWriter(f)
	if err != nil {
		return err
	}

	if _, err = cw.Write(data); err != nil {
		cw.Close()
		return err
	}

	if err = cw.Close(); err != nil {
		return err
	}

	closed = true
	return f.Close()
}
