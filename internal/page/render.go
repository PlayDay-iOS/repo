package page

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/fileutil"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

//go:embed templates/index.html.tmpl
var defaultLandingTemplate string

const suiteIndexTemplate = `<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>{{.Label}} Source</title></head>
  <body>
    <h1>{{.Label}} Source</h1>
    <p>Use this source line:</p>
    <pre>deb {{.RepoURL}}{{.Suite}}/ ./</pre>
  </body>
</html>
`

var suiteIndexTmpl = template.Must(template.New("suite-index").Parse(suiteIndexTemplate))

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
	RepoName     string
	RepoURL      string
	Suites       []SuiteInfo
	GeneratedAt  string
	Signed       bool
	HasPublicKey bool
}

// TitleCase converts a string to title case using English locale rules.
//
// A new Caser is allocated per call because cases.Caser is not safe for
// concurrent use of its String method (it mutates internal transformer state).
func TitleCase(s string) string {
	return cases.Title(language.English).String(s)
}

// RenderLandingPage renders the HTML landing page into outputDir/index.html.
// templatePath is optional: when empty, the embedded default template is used.
// hasPublicKey controls whether the repo-public.key download line is shown.
func RenderLandingPage(ctx context.Context, outputDir string, cfg *config.RepoConfig, templatePath string, buildTime time.Time, signed, hasPublicKey bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	tmplSrc := defaultLandingTemplate
	if templatePath != "" {
		b, err := os.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("reading template %s: %w", templatePath, err)
		}
		tmplSrc = string(b)
	}

	tmpl, err := template.New("index").Parse(tmplSrc)
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
		RepoName:     cfg.Name,
		RepoURL:      repoURL,
		Suites:       suites,
		GeneratedAt:  buildTime.UTC().Format("2006-01-02 15:04 UTC"),
		Signed:       signed,
		HasPublicKey: hasPublicKey,
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	return fileutil.WriteAtomic(filepath.Join(outputDir, "index.html"), 0644, func(w io.Writer) error {
		return tmpl.Execute(w, data)
	})
}

// WriteSuiteIndexHTML writes a simple info page for a suite directory.
func WriteSuiteIndexHTML(dir, suite, repoURL string) error {
	data := struct {
		Label   string
		Suite   string
		RepoURL string
	}{
		Label:   TitleCase(suite),
		Suite:   suite,
		RepoURL: repoURL,
	}
	return fileutil.WriteAtomic(filepath.Join(dir, "index.html"), 0644, func(w io.Writer) error {
		return suiteIndexTmpl.Execute(w, data)
	})
}
