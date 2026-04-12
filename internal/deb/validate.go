package deb

import "fmt"

// RequiredControlFields are mandatory for accepted packages across all ingest paths.
var RequiredControlFields = []string{
	"Package", "Version", "Architecture", "Maintainer", "Description",
}

// ValidateControl verifies required control fields and optional architecture allowlist.
func ValidateControl(control *ControlData, allowedArchitectures map[string]bool) error {
	for _, f := range RequiredControlFields {
		if control.Get(f) == "" {
			return fmt.Errorf("missing required control field %q", f)
		}
	}

	if len(allowedArchitectures) > 0 {
		arch := control.Get("Architecture")
		if !allowedArchitectures[arch] {
			return fmt.Errorf("architecture %q is not allowed", arch)
		}
	}

	return nil
}
