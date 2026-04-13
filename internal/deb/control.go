package deb

import (
	"fmt"
	"io"
	"strings"

	deblib "pault.ag/go/debian/deb"
)

// ControlData wraps a parsed control file from a .deb package.
// Keys in the internal values map are always lowercase for
// deterministic case-insensitive lookup. Order preserves the
// original case for RFC 822 output.
type ControlData struct {
	order  []string
	values map[string]string
}

// NewControlData creates a ControlData from ordered keys and their values
// keyed by canonical case. Only the provided keys are preserved; any extra
// entries in values are dropped so the stanza output always matches Order().
func NewControlData(keys []string, values map[string]string) *ControlData {
	lowered := make(map[string]string, len(keys))
	// Build a case-insensitive view of the input values once.
	caseFold := make(map[string]string, len(values))
	for k, v := range values {
		caseFold[strings.ToLower(k)] = v
	}
	for _, k := range keys {
		lowered[strings.ToLower(k)] = caseFold[strings.ToLower(k)]
	}
	return &ControlData{order: keys, values: lowered}
}

// Get returns the value for the given key (case-insensitive), or "".
func (c *ControlData) Get(key string) string {
	return c.values[strings.ToLower(key)]
}

// Order returns the field names in their original order and case.
func (c *ControlData) Order() []string {
	return c.order
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

	values := make(map[string]string, len(para.Values))
	for k, v := range para.Values {
		values[strings.ToLower(k)] = v
	}

	return &ControlData{order: para.Order, values: values}, nil
}
