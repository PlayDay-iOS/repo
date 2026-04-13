package page

import (
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"time"

	"github.com/PlayDay-iOS/repo/internal/config"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// SuiteInfo holds per-suite data for the landing page template.
type SuiteInfo struct {
	Name     string       // raw suite name, e.g. "stable"
	Label    string       // title-cased, e.g. "Stable"
	URL      string       // full suite URL, e.g. "https://example.com/repo/stable/"
	CydiaURL template.URL // trusted deeplink, e.g. "cydia://..."
	ZebraURL template.URL // trusted deeplink, e.g. "zbra://..."
	SileoURL template.URL // trusted deeplink, e.g. "sileo://..."
}

// TemplateData holds all values injected into the landing page template.
type TemplateData struct {
	RepoName    string
	RepoURL     string
	Suites      []SuiteInfo
	GeneratedAt string
}

var titleCaser = cases.Title(language.English)

// TitleCase converts a string to title case using English locale rules.
func TitleCase(s string) string {
	return titleCaser.String(s)
}

// RenderLandingPage renders the HTML landing page into outputDir/index.html.
func RenderLandingPage(outputDir string, cfg *config.RepoConfig, templatePath string, buildTime time.Time) error {
	tmplBytes, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("reading template %s: %w", templatePath, err)
	}

	tmpl, err := template.New("index").Parse(string(tmplBytes))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	repoURL := cfg.URL

	var suites []SuiteInfo
	for _, s := range cfg.Suites {
		suiteURL := repoURL + s + "/"
		suites = append(suites, SuiteInfo{
			Name:     s,
			Label:    TitleCase(s),
			URL:      suiteURL,
			CydiaURL: template.URL(CydiaDeeplink(suiteURL)),
			ZebraURL: template.URL(ZebraDeeplink(suiteURL)),
			SileoURL: template.URL(SileoDeeplink(suiteURL)),
		})
	}

	data := TemplateData{
		RepoName:    cfg.Name,
		RepoURL:     repoURL,
		Suites:      suites,
		GeneratedAt: buildTime.UTC().Format("2006-01-02 15:04 UTC"),
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	return writeFileAtomic(filepath.Join(outputDir, "index.html"), 0644, func(f *os.File) error {
		return tmpl.Execute(f, data)
	})
}

// WriteSuiteIndexHTML writes a simple info page for a suite directory.
func WriteSuiteIndexHTML(dir, suite, repoURL string) error {
	escapedSuite := html.EscapeString(TitleCase(suite))
	escapedURL := html.EscapeString(repoURL)
	escapedRawSuite := html.EscapeString(suite)
	content := fmt.Sprintf(`<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>%s Source</title></head>
  <body>
    <h1>%s Source</h1>
    <p>Use this source line:</p>
    <pre>deb %s%s/ ./</pre>
  </body>
</html>
`, escapedSuite, escapedSuite, escapedURL, escapedRawSuite)

	return writeFileAtomic(filepath.Join(dir, "index.html"), 0644, func(f *os.File) error {
		_, err := f.WriteString(content)
		return err
	})
}

// writeFileAtomic writes via a temp file in the target directory and renames on success.
func writeFileAtomic(path string, perm os.FileMode, writeFn func(*os.File) error) (err error) {
	tmpPath := path + ".tmp"
	f, err := os.OpenFile(tmpPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if err = writeFn(f); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}
