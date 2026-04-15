package depiction

import (
	"net/url"
	"testing"
)

func TestPackageDepictionURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		repo     string
		baseName string
		want     string
	}{
		{
			name:     "plain basename",
			repo:     "https://example.com/repo/",
			baseName: "test-1.0",
			want:     "https://example.com/repo/depictions/test-1.0/depiction.html",
		},
		{
			name:     "basename with dots and dashes",
			repo:     "https://example.com/repo/",
			baseName: "us.hackulo.appsync50plus-2.1-legacy",
			want:     "https://example.com/repo/depictions/us.hackulo.appsync50plus-2.1-legacy/depiction.html",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PackageDepictionURL(tc.repo, tc.baseName)
			if got != tc.want {
				t.Errorf("PackageDepictionURL() = %q, want %q", got, tc.want)
			}
			if _, err := url.Parse(got); err != nil {
				t.Errorf("url.Parse(%q) failed: %v", got, err)
			}
		})
	}
}

func TestPackageSileoURL(t *testing.T) {
	t.Parallel()
	got := PackageSileoURL("https://example.com/repo/", "us.hackulo.appsync50plus-2.1-legacy")
	want := "https://example.com/repo/depictions/us.hackulo.appsync50plus-2.1-legacy/sileo.json"
	if got != want {
		t.Errorf("PackageSileoURL() = %q, want %q", got, want)
	}
	if _, err := url.Parse(got); err != nil {
		t.Errorf("url.Parse(%q) failed: %v", got, err)
	}
}
