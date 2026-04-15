package depiction

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// firmwareClauseRe matches a single firmware dependency clause inside a
// Depends value, e.g. "firmware (>= 14.0)". Operators: >=, <=, >>, <<.
var firmwareClauseRe = regexp.MustCompile(`firmware\s*\(\s*(>=|<=|>>|<<)\s*([0-9]+(?:\.[0-9]+)*)\s*\)`)

// iosVersion is a two-component iOS version (major.minor). Extra segments
// beyond minor are ignored — iOS banners never show them.
type iosVersion struct {
	Major, Minor int
}

func (v iosVersion) less(o iosVersion) bool {
	if v.Major != o.Major {
		return v.Major < o.Major
	}
	return v.Minor < o.Minor
}

func (v iosVersion) equal(o iosVersion) bool {
	return v.Major == o.Major && v.Minor == o.Minor
}

func (v iosVersion) String() string {
	return fmt.Sprintf("%d.%d", v.Major, v.Minor)
}

// parseIOSVersion normalises a dotted version to major.minor. Bare majors
// like "14" become {14, 0}. Versions beyond minor are truncated.
func parseIOSVersion(s string) (iosVersion, error) {
	parts := strings.Split(s, ".")
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return iosVersion{}, err
	}
	minor := 0
	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return iosVersion{}, err
		}
	}
	return iosVersion{Major: major, Minor: minor}, nil
}

// ParseCompat derives a banner string from a Depends value and an optional
// X-Supported-iOS override. Override wins verbatim when non-whitespace.
// Returns ("", false) when no banner should be displayed.
func ParseCompat(depends, override string) (string, bool) {
	if s := strings.TrimSpace(override); s != "" {
		return s, true
	}

	var (
		lo, hi         *iosVersion
		loIncl, hiIncl bool
	)
	for _, m := range firmwareClauseRe.FindAllStringSubmatch(depends, -1) {
		op, verStr := m[1], m[2]
		v, err := parseIOSVersion(verStr)
		if err != nil {
			// regex limits to digits and dots, but keep the guard for safety.
			continue
		}
		switch op {
		case ">=":
			if lo == nil || lo.less(v) {
				lo = &v
				loIncl = true
			}
		case ">>":
			if lo == nil || lo.less(v) {
				lo = &v
				loIncl = false
			}
		case "<=":
			if hi == nil || v.less(*hi) {
				hi = &v
				hiIncl = true
			}
		case "<<":
			if hi == nil || v.less(*hi) {
				hi = &v
				hiIncl = false
			}
		}
	}

	if lo == nil && hi == nil {
		return "", false
	}

	// Contradiction: low bound above (or equal-to-exclusive) high bound.
	if lo != nil && hi != nil {
		if hi.less(*lo) {
			return "", false
		}
		if lo.equal(*hi) && !(loIncl && hiIncl) {
			return "", false
		}
	}

	switch {
	case lo != nil && hi != nil && hiIncl:
		return fmt.Sprintf("iOS %s – %s", lo, hi), true
	case lo != nil && hi != nil && !hiIncl:
		return fmt.Sprintf("iOS %s – %s", lo, shiftDown(*hi)), true
	case lo != nil:
		// Lower bound only. We render inclusive and exclusive lower bounds
		// identically because "iOS 14.0+" is how users read both.
		return fmt.Sprintf("iOS %s+", lo), true
	case hiIncl:
		return fmt.Sprintf("up to iOS %s", hi), true
	default:
		return fmt.Sprintf("up to iOS %s", shiftDown(*hi)), true
	}
}

// shiftDown converts an exclusive upper bound to its inclusive display form.
// 17.0 -> "16.x"; 16.4 -> "16.3"; 16.0 -> "15.x" (major rollover).
func shiftDown(v iosVersion) string {
	if v.Minor == 0 {
		return fmt.Sprintf("%d.x", v.Major-1)
	}
	return fmt.Sprintf("%d.%d", v.Major, v.Minor-1)
}
