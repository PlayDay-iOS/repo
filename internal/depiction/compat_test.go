package depiction

import "testing"

func TestParseCompat_OverrideWinsVerbatim(t *testing.T) {
	t.Parallel()
	got, ok := ParseCompat("firmware (>= 14.0)", "iOS 15 only")
	if !ok || got != "iOS 15 only" {
		t.Errorf("ParseCompat override: got (%q, %v), want (iOS 15 only, true)", got, ok)
	}
}

func TestParseCompat_OverrideWhitespaceTreatedAsAbsent(t *testing.T) {
	t.Parallel()
	got, ok := ParseCompat("firmware (>= 14.0)", "   ")
	if !ok || got != "iOS 14.0+" {
		t.Errorf("whitespace override: got (%q, %v), want (iOS 14.0+, true)", got, ok)
	}
}

func TestParseCompat_Formatter(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, depends, want string
		wantOK              bool
	}{
		{"min+exclusive max major", "firmware (>= 14.0), firmware (<< 17.0)", "iOS 14.0 – 16.x", true},
		{"min+exclusive max with minor", "firmware (>= 14.0), firmware (<< 16.4)", "iOS 14.0 – 16.3", true},
		{"min+inclusive max", "firmware (>= 14.0), firmware (<= 16.7)", "iOS 14.0 – 16.7", true},
		{"min only", "firmware (>= 14.0)", "iOS 14.0+", true},
		{"max only inclusive", "firmware (<= 16.7)", "up to iOS 16.7", true},
		{"max only exclusive", "firmware (<< 17.0)", "up to iOS 16.x", true},
		{"no firmware clause", "some-other-dep (>= 1.0)", "", false},
		{"empty depends", "", "", false},
		{"bare major normalised", "firmware (>= 14)", "iOS 14.0+", true},
		{"contradictory bounds", "firmware (>= 16.0), firmware (<< 14.0)", "", false},
		{"multiple >=, strictest (highest) wins", "firmware (>= 14.0), firmware (>= 15.0)", "iOS 15.0+", true},
		{"multiple <<, strictest (lowest) wins", "firmware (<< 17.0), firmware (<< 16.0)", "up to iOS 15.x", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := ParseCompat(tc.depends, "")
			if ok != tc.wantOK || got != tc.want {
				t.Errorf("ParseCompat(%q, \"\") = (%q, %v), want (%q, %v)",
					tc.depends, got, ok, tc.want, tc.wantOK)
			}
		})
	}
}

func TestParseCompat_MalformedClauseSkipped(t *testing.T) {
	t.Parallel()
	// A malformed firmware clause is ignored by the regex (no match);
	// other valid clauses still contribute.
	got, ok := ParseCompat("firmware (>= abc), firmware (>= 14.0)", "")
	if !ok || got != "iOS 14.0+" {
		t.Errorf("ParseCompat with one malformed clause: got (%q, %v), want (iOS 14.0+, true)", got, ok)
	}
}
