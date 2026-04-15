package depiction

import (
	"net/url"
	"testing"
)

func TestPackageDepictionURL(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name            string
		repo, pkg, ver  string
		want            string
	}{
		{
			name: "plain",
			repo: "https://example.com/repo/",
			pkg:  "com.foo.bar",
			ver:  "1.0",
			want: "https://example.com/repo/depictions/com.foo.bar/1.0/depiction.html",
		},
		{
			name: "epoch version is percent-escaped",
			repo: "https://example.com/repo/",
			pkg:  "com.foo.bar",
			ver:  "1:1.8r-260",
			want: "https://example.com/repo/depictions/com.foo.bar/1%3A1.8r-260/depiction.html",
		},
		{
			name: "tilde passes through (unreserved per RFC 3986)",
			repo: "https://example.com/repo/",
			pkg:  "com.foo.bar",
			ver:  "1.0~rc1",
			want: "https://example.com/repo/depictions/com.foo.bar/1.0~rc1/depiction.html",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := PackageDepictionURL(tc.repo, tc.pkg, tc.ver)
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
	got := PackageSileoURL("https://example.com/repo/", "com.foo.bar", "1:1.8r-260")
	want := "https://example.com/repo/depictions/com.foo.bar/1%3A1.8r-260/sileo.json"
	if got != want {
		t.Errorf("PackageSileoURL() = %q, want %q", got, want)
	}
	if _, err := url.Parse(got); err != nil {
		t.Errorf("url.Parse(%q) failed: %v", got, err)
	}
}
