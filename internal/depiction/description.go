package depiction

import (
	"html"
	"html/template"
	"strings"
)

// RenderDescription converts a Debian multi-line Description field to HTML.
// The first line is the synopsis, wrapped in <p class="synopsis">. Subsequent
// lines follow the Debian convention: each begins with a leading space; a
// single " ." on its own marks a paragraph break. Lines within a paragraph
// are joined with a single space. All output is HTML-escaped.
func RenderDescription(raw string) template.HTML {
	if raw == "" {
		return ""
	}
	lines := strings.Split(raw, "\n")

	var b strings.Builder
	synopsis := strings.TrimSpace(lines[0])
	if synopsis != "" {
		b.WriteString(`<p class="synopsis">`)
		b.WriteString(html.EscapeString(synopsis))
		b.WriteString("</p>\n")
	}

	var para []string
	flush := func() {
		if len(para) == 0 {
			return
		}
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(strings.Join(para, " ")))
		b.WriteString("</p>\n")
		para = para[:0]
	}

	for _, line := range lines[1:] {
		trimmed := strings.TrimPrefix(line, " ")
		if trimmed == "." {
			flush()
			continue
		}
		text := strings.TrimSpace(trimmed)
		if text == "" {
			continue
		}
		para = append(para, text)
	}
	flush()

	return template.HTML(b.String())
}
