package fileutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteAtomic writes a file at path by creating a temp file in the same
// directory, calling writeFn to populate its contents, then renaming it
// into place. On any error, the temp file is removed.
//
// The callback receives an io.Writer; callers should not retain it after
// writeFn returns.
func WriteAtomic(path string, perm os.FileMode, writeFn func(io.Writer) error) (err error) {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	f, err := os.CreateTemp(dir, base+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()

	cleanup := func() {
		os.Remove(tmpPath)
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	if err = writeFn(f); err != nil {
		f.Close()
		return err
	}
	if err = f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return err
	}
	return nil
}

// WriteAtomicBytes is a convenience wrapper for writing a fixed byte slice.
func WriteAtomicBytes(path string, perm os.FileMode, data []byte) error {
	return WriteAtomic(path, perm, func(w io.Writer) error {
		_, err := w.Write(data)
		return err
	})
}
