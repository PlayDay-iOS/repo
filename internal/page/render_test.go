package page

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/PlayDay-iOS/repo/internal/config"
)

func TestRenderLandingPage(t *testing.T) {
	dir := t.TempDir()
	tmplDir := filepath.Join(dir, "templates")
	os.MkdirAll(tmplDir, 0755)

	tmpl := `<html><title>{{.RepoName}}</title><body>{{range .Suites}}<p>deb {{.URL}} ./</p>{{end}}{{range .Suites}}<a href="{{.CydiaURL}}">Cydia</a><a href="{{.ZebraURL}}">Zebra</a><a href="{{.SileoURL}}">Sileo</a>{{end}}</body></html>`
	tmplPath := filepath.Join(tmplDir, "index.html.tmpl")
	os.WriteFile(tmplPath, []byte(tmpl), 0644)

	cfg := &config.RepoConfig{
		Name:   "Test Repo",
		URL:    "https://example.com/repo/",
		Suites: []string{"stable", "beta"},
	}

	outDir := filepath.Join(dir, "out")
	if err := RenderLandingPage(outDir, cfg, tmplPath, time.Now()); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)

	if !strings.Contains(html, "<title>Test Repo</title>") {
		t.Error("missing repo name in title")
	}
	if !strings.Contains(html, "deb https://example.com/repo/stable/ ./") {
		t.Error("missing stable APT line")
	}
	if !strings.Contains(html, "deb https://example.com/repo/beta/ ./") {
		t.Error("missing beta APT line")
	}
	if !strings.Contains(html, "cydia://") {
		t.Error("missing Cydia deeplink")
	}
	if !strings.Contains(html, "zbra://") {
		t.Error("missing Zebra deeplink")
	}
	if !strings.Contains(html, "sileo://") {
		t.Error("missing Sileo deeplink")
	}
}

func TestWriteSuiteIndexHTML(t *testing.T) {
	dir := t.TempDir()
	if err := WriteSuiteIndexHTML(dir, "stable", "https://example.com/repo/"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)

	if !strings.Contains(html, "deb https://example.com/repo/stable/ ./") {
		t.Error("missing source line")
	}
}

func TestLandingTemplateHasCollapsibleDetailsMenus(t *testing.T) {
	dir := t.TempDir()
	// test working dir is the package dir (internal/page); template lives two levels up
	templatePath := filepath.Join("..", "..", "templates", "index.html.tmpl")

	cfg := &config.RepoConfig{
		Name:   "Test Repo",
		URL:    "https://example.com/repo/",
		Suites: []string{"stable", "beta"},
	}

	outDir := filepath.Join(dir, "out")
	if err := RenderLandingPage(outDir, cfg, templatePath, time.Now()); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(data)

	checks := []string{
		`id="menu-cydia"`,
		`<summary>Add to Cydia</summary>`,
		`id="menu-zebra"`,
		`<summary>Add to Zebra</summary>`,
		`id="menu-sileo"`,
		`<summary>Add to Sileo</summary>`,
		`Tap sections to expand source options.`,
	}

	for _, check := range checks {
		if !strings.Contains(html, check) {
			t.Errorf("missing accordion marker %q", check)
		}
	}
}
