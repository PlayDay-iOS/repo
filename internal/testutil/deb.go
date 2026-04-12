package testutil

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"time"
)

// Field is a key-value pair for building test .deb control files.
type Field struct {
	Key, Value string
}

// BuildMinimalDeb constructs a minimal .deb (ar archive) in memory.
func BuildMinimalDeb(fields []Field) []byte {
	var controlBuf bytes.Buffer
	for _, f := range fields {
		fmt.Fprintf(&controlBuf, "%s: %s\n", f.Key, f.Value)
	}
	controlBytes := controlBuf.Bytes()

	var tarBuf bytes.Buffer
	gw := gzip.NewWriter(&tarBuf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name:    "./control",
		Size:    int64(len(controlBytes)),
		Mode:    0644,
		ModTime: time.Unix(0, 0),
	}); err != nil {
		panic(fmt.Sprintf("testutil: writing tar header: %v", err))
	}
	if _, err := tw.Write(controlBytes); err != nil {
		panic(fmt.Sprintf("testutil: writing tar data: %v", err))
	}
	if err := tw.Close(); err != nil {
		panic(fmt.Sprintf("testutil: closing tar: %v", err))
	}
	if err := gw.Close(); err != nil {
		panic(fmt.Sprintf("testutil: closing gzip: %v", err))
	}
	controlTar := tarBuf.Bytes()

	var dataBuf bytes.Buffer
	dgw := gzip.NewWriter(&dataBuf)
	dtw := tar.NewWriter(dgw)
	dtw.Close()
	dgw.Close()
	dataTar := dataBuf.Bytes()

	debianBinary := []byte("2.0\n")

	var ar bytes.Buffer
	ar.WriteString("!<arch>\n")
	writeArEntry(&ar, "debian-binary", debianBinary)
	writeArEntry(&ar, "control.tar.gz", controlTar)
	writeArEntry(&ar, "data.tar.gz", dataTar)

	return ar.Bytes()
}

func writeArEntry(buf *bytes.Buffer, name string, data []byte) {
	header := make([]byte, 60)
	copy(header[0:16], padRight(name, 16))
	copy(header[16:28], padRight("0", 12))
	copy(header[28:34], padRight("0", 6))
	copy(header[34:40], padRight("0", 6))
	copy(header[40:48], padRight("100644", 8))
	copy(header[48:58], padRight(fmt.Sprintf("%d", len(data)), 10))
	header[58] = 0x60
	header[59] = 0x0a
	buf.Write(header)
	buf.Write(data)
	if len(data)%2 != 0 {
		buf.WriteByte('\n')
	}
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	pad := make([]byte, length)
	copy(pad, s)
	for i := len(s); i < length; i++ {
		pad[i] = ' '
	}
	return string(pad)
}
