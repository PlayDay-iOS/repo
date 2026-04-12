package validate

import "regexp"

// Name matches safe identifiers (suite names, component names, repo names):
// must start with alphanumeric, then alphanumeric, dash, underscore, or dot.
var Name = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]*$`)
