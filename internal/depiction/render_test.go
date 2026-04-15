package depiction

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
)

func newEntry(fields map[string]string) *deb.PackageEntry {
	keys := []string{"Package", "Version", "Architecture", "Maintainer", "Description"}
	for k := range fields {
		found := false
		for _, kk := range keys {
			if kk == k {
				found = true
				break
			}
		}
		if !found {
			keys = append(keys, k)
		}
	}
	return &deb.PackageEntry{Control: deb.NewControlData(keys, fields)}
}

func TestRender_WritesExpectedTree(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	entries := []*deb.PackageEntry{
		newEntry(map[string]string{
			"Package":      "com.foo.bar",
			"Version":      "1.0",
			"Architecture": "iphoneos-arm64",
			"Maintainer":   "Foo",
			"Description":  "A foo package",
			"Depends":      "firmware (>= 14.0), firmware (<< 17.0)",
		}),
	}

	if err := Render(context.Background(), outDir, entries, cfg, Options{}); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	htmlPath := filepath.Join(outDir, "depictions", "com.foo.bar", "1.0", "depiction.html")
	jsonPath := filepath.Join(outDir, "depictions", "com.foo.bar", "1.0", "sileo.json")
	if _, err := os.Stat(htmlPath); err != nil {
		t.Errorf("missing depiction.html: %v", err)
	}
	if _, err := os.Stat(jsonPath); err != nil {
		t.Errorf("missing sileo.json: %v", err)
	}

	htmlBytes, _ := os.ReadFile(htmlPath)
	html := string(htmlBytes)
	if !strings.Contains(html, "<title>com.foo.bar</title>") {
		t.Errorf("html should use Package as fallback title")
	}
	if !strings.Contains(html, `href="https://example.com/repo/depictions/style.css"`) {
		t.Errorf("html should link absolute style.css")
	}
	if !strings.Contains(html, "iOS 14.0 – 16.x") {
		t.Errorf("html should contain compat banner, got: %s", html)
	}
}

func TestRender_EpochVersionEscapedInPath(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	entries := []*deb.PackageEntry{
		newEntry(map[string]string{
			"Package":      "com.foo.bar",
			"Version":      "1:1.8r-260",
			"Architecture": "iphoneos-arm",
			"Maintainer":   "x",
			"Description":  "x",
		}),
	}

	if err := Render(context.Background(), outDir, entries, cfg, Options{}); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	escapedDir := filepath.Join(outDir, "depictions", "com.foo.bar", "1%3A1.8r-260")
	if _, err := os.Stat(filepath.Join(escapedDir, "depiction.html")); err != nil {
		t.Errorf("expected percent-escaped version dir, stat err: %v", err)
	}
}

func TestRender_DeduplicatesIdenticalPairAcrossSuites(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	// Two entries with identical control — simulates same .deb in two suites.
	fields := map[string]string{
		"Package": "com.foo.bar", "Version": "1.0",
		"Architecture": "iphoneos-arm64", "Maintainer": "x", "Description": "x",
	}
	entries := []*deb.PackageEntry{newEntry(fields), newEntry(fields)}

	if err := Render(context.Background(), outDir, entries, cfg, Options{}); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	// Only one file written (idempotent).
	if _, err := os.Stat(filepath.Join(outDir, "depictions", "com.foo.bar", "1.0", "depiction.html")); err != nil {
		t.Errorf("expected single depiction, got: %v", err)
	}
}

func TestRender_FailsOnDivergentContentForSamePair(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	a := newEntry(map[string]string{
		"Package": "com.foo.bar", "Version": "1.0",
		"Architecture": "iphoneos-arm64", "Maintainer": "a", "Description": "one",
	})
	b := newEntry(map[string]string{
		"Package": "com.foo.bar", "Version": "1.0",
		"Architecture": "iphoneos-arm64", "Maintainer": "b", "Description": "two",
	})

	err := Render(context.Background(), outDir, []*deb.PackageEntry{a, b}, cfg, Options{})
	if err == nil || !strings.Contains(err.Error(), "depiction content mismatch") {
		t.Fatalf("expected content-mismatch error, got: %v", err)
	}
}

func TestRender_EmptyEntriesIsNoop(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}

	if err := Render(context.Background(), outDir, nil, cfg, Options{}); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outDir, "depictions")); !os.IsNotExist(err) {
		t.Errorf("depictions/ should not exist when entries empty")
	}
}

func TestRender_TemplateOverrideUsed(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	tmplDir := t.TempDir()
	overridePath := filepath.Join(tmplDir, "override.tmpl")
	if err := os.WriteFile(overridePath, []byte("OVERRIDE: {{.Name}}"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	entries := []*deb.PackageEntry{
		newEntry(map[string]string{
			"Package": "foo", "Version": "1.0",
			"Architecture": "iphoneos-arm64", "Maintainer": "x", "Description": "x",
		}),
	}

	if err := Render(context.Background(), outDir, entries, cfg, Options{TemplatePath: overridePath}); err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(outDir, "depictions", "foo", "1.0", "depiction.html"))
	if string(b) != "OVERRIDE: foo" {
		t.Errorf("expected override content, got %q", string(b))
	}
}

func TestWriteStylesheet_WritesDefault(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	if err := WriteStylesheet(outDir, ""); err != nil {
		t.Fatalf("WriteStylesheet failed: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(outDir, "depictions", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	if !strings.Contains(string(b), "--ink") {
		t.Errorf("style.css did not look like embedded default, got: %q", string(b))
	}
}

func TestWriteStylesheet_OverridePathUsed(t *testing.T) {
	t.Parallel()
	outDir := t.TempDir()
	override := filepath.Join(t.TempDir(), "custom.css")
	if err := os.WriteFile(override, []byte("body { color: red; }"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := WriteStylesheet(outDir, override); err != nil {
		t.Fatalf("WriteStylesheet failed: %v", err)
	}
	b, _ := os.ReadFile(filepath.Join(outDir, "depictions", "style.css"))
	if string(b) != "body { color: red; }" {
		t.Errorf("override not applied, got %q", string(b))
	}
}
