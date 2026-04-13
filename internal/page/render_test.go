package page

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PlayDay-iOS/repo/internal/config"
)

func TestRenderLandingPage_FileTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, "templates")
	if err := os.MkdirAll(tmplDir, 0755); err != nil {
		t.Fatal(err)
	}

	tmpl := `<html><title>{{.RepoName}}</title><body>{{range .Suites}}<p>deb {{.URL}} ./</p>{{end}}{{range .Suites}}<a href="{{.CydiaURL}}">Cydia</a><a href="{{.ZebraURL}}">Zebra</a><a href="{{.SileoURL}}">Sileo</a>{{end}}</body></html>`
	tmplPath := filepath.Join(tmplDir, "index.html.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmpl), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.RepoConfig{
		Name:   "Test Repo",
		URL:    "https://example.com/repo/",
		Suites: []string{"stable", "beta"},
	}

	outDir := filepath.Join(dir, "out")
	if err := RenderLandingPage(context.Background(), outDir, cfg, tmplPath, time.Now(), false, false); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)

	for _, want := range []string{
		"<title>Test Repo</title>",
		"deb https://example.com/repo/stable/ ./",
		"deb https://example.com/repo/beta/ ./",
		"cydia://",
		"zbra://",
		"sileo://",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q in rendered page", want)
		}
	}
}

func TestWriteSuiteIndexHTML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := WriteSuiteIndexHTML(dir, "stable", "https://example.com/repo/"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "deb https://example.com/repo/stable/ ./") {
		t.Error("missing source line")
	}
}

func TestDefaultTemplate_RendersKeyMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := &config.RepoConfig{
		Name:   "Test Repo",
		URL:    "https://example.com/repo/",
		Suites: []string{"stable", "beta"},
	}

	outDir := filepath.Join(dir, "out")
	if err := RenderLandingPage(context.Background(), outDir, cfg, "", time.Now(), true, true); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)

	for _, check := range []string{
		`id="menu-cydia"`,
		`<summary>Add to Cydia</summary>`,
		`id="menu-zebra"`,
		`<summary>Add to Zebra</summary>`,
		`id="menu-sileo"`,
		`<summary>Add to Sileo</summary>`,
		`Tap sections to expand source options.`,
		"InRelease",       // signed variant should appear when Signed=true
		"repo-public.key", // public-key line should appear when HasPublicKey=true
	} {
		if !strings.Contains(html, check) {
			t.Errorf("missing marker %q", check)
		}
	}
}

func TestDefaultTemplate_OmitsInReleaseWhenUnsigned(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	cfg := &config.RepoConfig{
		Name:   "Test Repo",
		URL:    "https://example.com/repo/",
		Suites: []string{"stable"},
	}

	outDir := filepath.Join(dir, "out")
	if err := RenderLandingPage(context.Background(), outDir, cfg, "", time.Now(), false, false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)
	if strings.Contains(html, "InRelease") {
		t.Error("InRelease should be omitted when signing is disabled")
	}
	if strings.Contains(html, "repo-public.key") {
		t.Error("repo-public.key link should be omitted when HasPublicKey is false")
	}
}
