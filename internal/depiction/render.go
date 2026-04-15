package depiction

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
	"github.com/PlayDay-iOS/repo/internal/fileutil"
)

//go:embed templates/depiction.html.tmpl
var defaultHTMLTemplate string

//go:embed templates/style.css
var defaultStyle []byte

// Options configures Render and WriteStylesheet. Empty fields mean
// "use the embedded default."
type Options struct {
	TemplatePath string
	StylePath    string
}

// templateData is the struct bound to depiction.html.tmpl.
type templateData struct {
	Name            string
	StyleURL        string
	Package         string
	Version         string
	Architecture    string
	Maintainer      string
	Section         string
	InstalledSize   string
	Depends         string
	Compat          string
	DescriptionHTML template.HTML
}

// Render writes per-deb depiction.html and sileo.json files under
// outputDir/depictions/<basename>/. Entries with the same basename are
// deduplicated; if two such entries produce different serialized output,
// the build fails.
//
// When entries is empty, Render is a no-op and creates no directories.
func Render(ctx context.Context, outputDir string, entries []*deb.PackageEntry, cfg *config.RepoConfig, opts Options) error {
	if len(entries) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	tmplSrc := defaultHTMLTemplate
	if opts.TemplatePath != "" {
		b, err := os.ReadFile(opts.TemplatePath)
		if err != nil {
			return fmt.Errorf("reading depiction template %s: %w", opts.TemplatePath, err)
		}
		tmplSrc = string(b)
	}
	tmpl, err := template.New("depiction").Parse(tmplSrc)
	if err != nil {
		return fmt.Errorf("parsing depiction template: %w", err)
	}

	styleURL := cfg.URL + "depictions/style.css"

	type rendered struct {
		html []byte
		json []byte
	}
	seen := make(map[string]rendered, len(entries))

	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		bn := entryBaseName(e)

		data := buildTemplateData(e, styleURL)
		var htmlBuf bytes.Buffer
		if err := tmpl.Execute(&htmlBuf, data); err != nil {
			return fmt.Errorf("rendering depiction for %s: %w", bn, err)
		}
		jsonBytes, err := buildSileoBytes(e, data.Compat)
		if err != nil {
			return fmt.Errorf("building sileo JSON for %s: %w", bn, err)
		}
		cur := rendered{html: htmlBuf.Bytes(), json: jsonBytes}

		if prev, exists := seen[bn]; exists {
			if !bytes.Equal(prev.html, cur.html) || !bytes.Equal(prev.json, cur.json) {
				return fmt.Errorf("depiction content mismatch for %s", bn)
			}
			continue
		}
		seen[bn] = cur

		dir := filepath.Join(outputDir, "depictions", bn)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating depiction dir for %s: %w", bn, err)
		}
		if err := fileutil.WriteAtomicBytes(filepath.Join(dir, "depiction.html"), 0644, cur.html); err != nil {
			return fmt.Errorf("writing depiction.html for %s: %w", bn, err)
		}
		if err := fileutil.WriteAtomicBytes(filepath.Join(dir, "sileo.json"), 0644, cur.json); err != nil {
			return fmt.Errorf("writing sileo.json for %s: %w", bn, err)
		}
	}
	return nil
}

// WriteStylesheet writes outputDir/depictions/style.css using either the
// override file at stylePath or the embedded default.
func WriteStylesheet(outputDir, stylePath string) error {
	data := defaultStyle
	if stylePath != "" {
		b, err := os.ReadFile(stylePath)
		if err != nil {
			return fmt.Errorf("reading depiction style %s: %w", stylePath, err)
		}
		data = b
	}
	dir := filepath.Join(outputDir, "depictions")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating depictions dir: %w", err)
	}
	return fileutil.WriteAtomicBytes(filepath.Join(dir, "style.css"), 0644, data)
}

func buildTemplateData(e *deb.PackageEntry, styleURL string) templateData {
	pkg := e.Control.Get("Package")
	name := e.Control.Get("Name")
	if name == "" {
		name = pkg
	}
	compat, _ := ParseCompat(e.Control.Get("Depends"), e.Control.Get("X-Supported-iOS"))
	return templateData{
		Name:            name,
		StyleURL:        styleURL,
		Package:         pkg,
		Version:         e.Control.Get("Version"),
		Architecture:    e.Control.Get("Architecture"),
		Maintainer:      e.Control.Get("Maintainer"),
		Section:         e.Control.Get("Section"),
		InstalledSize:   e.Control.Get("Installed-Size"),
		Depends:         e.Control.Get("Depends"),
		Compat:          compat,
		DescriptionHTML: RenderDescription(e.Control.Get("Description")),
	}
}

func buildSileoBytes(e *deb.PackageEntry, compat string) ([]byte, error) {
	pkg := e.Control.Get("Package")
	name := e.Control.Get("Name")
	if name == "" {
		name = pkg
	}
	return BuildSileoJSON(SileoEntry{
		DisplayName:   name,
		Section:       e.Control.Get("Section"),
		Compat:        compat,
		Description:   e.Control.Get("Description"),
		Version:       e.Control.Get("Version"),
		Architecture:  e.Control.Get("Architecture"),
		Maintainer:    e.Control.Get("Maintainer"),
		InstalledSize: e.Control.Get("Installed-Size"),
		Depends:       e.Control.Get("Depends"),
	})
}
