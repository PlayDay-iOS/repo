package deb

import (
	"fmt"
	"io"

	"pault.ag/go/debian/control"
	deblib "pault.ag/go/debian/deb"
)

// ControlData wraps a parsed control file from a .deb package.
type ControlData struct {
	paragraph control.Paragraph
}

// NewControlData creates a ControlData from key-value pairs (for testing).
func NewControlData(keys []string, values map[string]string) *ControlData {
	return &ControlData{paragraph: control.Paragraph{Order: keys, Values: values}}
}

// Get returns the value for the exact key name, or "".
func (c *ControlData) Get(key string) string {
	return c.paragraph.Values[key]
}

// Order returns the field names in their original order.
func (c *ControlData) Order() []string {
	return c.paragraph.Order
}

// ExtractControlFromReader reads a .deb (ar archive) from an io.ReaderAt and parses the control file.
func ExtractControlFromReader(r io.ReaderAt, name string) (*ControlData, error) {
	pkg, err := deblib.Load(r, name)
	if err != nil {
		return nil, fmt.Errorf("loading .deb: %w", err)
	}
	defer pkg.Close()

	para := pkg.Control.Paragraph
	if len(para.Order) == 0 {
		return nil, fmt.Errorf("control file has no fields")
	}

	return &ControlData{paragraph: para}, nil
}
