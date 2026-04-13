package fileutil

import (
	"io"
	"os"
)

// CopyFile copies src to dst, creating or truncating dst with mode 0644
// (the canonical mode for files published into the repository output).
// The destination is fsynced before close.
func CopyFile(src, dst string) error {
	srcF, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcF.Close()

	dstF, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}

	return finishCopy(srcF, dstF, dst, false)
}

// CopyFileExclusive copies src to dst with mode 0644, failing if dst
// already exists (O_EXCL). The destination is fsynced before close.
// On copy failure the partial dst is removed.
func CopyFileExclusive(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		return err
	}

	return finishCopy(in, out, dst, true)
}

func finishCopy(src io.Reader, dst *os.File, dstPath string, removeOnError bool) error {
	_, copyErr := io.Copy(dst, src)
	syncErr := dst.Sync()
	closeErr := dst.Close()

	for _, e := range []error{copyErr, syncErr, closeErr} {
		if e != nil {
			if removeOnError {
				os.Remove(dstPath)
			}
			return e
		}
	}
	return nil
}
