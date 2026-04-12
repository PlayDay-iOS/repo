package validate

import "testing"

func TestName_Valid(t *testing.T) {
	t.Parallel()
	valid := []string{
		"stable",
		"beta",
		"main",
		"iphoneos-arm64",
		"my.repo",
		"my_repo",
		"my-repo",
		"A123",
		"x",
		"0day",
		"repo.name-with_mixed.chars",
	}
	for _, name := range valid {
		t.Run(name, func(t *testing.T) {
			if !Name.MatchString(name) {
				t.Errorf("expected %q to be valid", name)
			}
		})
	}
}

func TestName_Invalid(t *testing.T) {
	t.Parallel()
	invalid := []string{
		"",
		"-leading-dash",
		".leading-dot",
		"_leading-underscore",
		"has space",
		"has/slash",
		"has\nnewline",
		"has\ttab",
		"has@symbol",
		"../traversal",
	}
	for _, name := range invalid {
		t.Run(name, func(t *testing.T) {
			if Name.MatchString(name) {
				t.Errorf("expected %q to be invalid", name)
			}
		})
	}
}
