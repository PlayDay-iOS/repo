package depiction

import (
	"strings"
	"testing"
)

func TestRenderDescription_SynopsisAndParagraphs(t *testing.T) {
	t.Parallel()
	raw := "Short synopsis\n First paragraph line one.\n First paragraph line two.\n .\n Second paragraph."
	got := string(RenderDescription(raw))

	want := `<p class="synopsis">Short synopsis</p>
<p>First paragraph line one. First paragraph line two.</p>
<p>Second paragraph.</p>
`
	if got != want {
		t.Errorf("RenderDescription mismatch:\n got:  %q\n want: %q", got, want)
	}
}

func TestRenderDescription_EscapesHTML(t *testing.T) {
	t.Parallel()
	got := string(RenderDescription("Line with <script>alert(1)</script>\n Body <b>bold</b>"))
	if strings.Contains(got, "<script>") {
		t.Errorf("expected synopsis to be escaped, got %q", got)
	}
	if strings.Contains(got, "<b>") {
		t.Errorf("expected body <b> to be escaped, got %q", got)
	}
	if !strings.Contains(got, "&lt;script&gt;") {
		t.Errorf("expected escaped &lt;script&gt;, got %q", got)
	}
}

func TestRenderDescription_EmptyInput(t *testing.T) {
	t.Parallel()
	if got := string(RenderDescription("")); got != "" {
		t.Errorf("RenderDescription(\"\") = %q, want empty", got)
	}
}

func TestRenderDescription_SynopsisOnly(t *testing.T) {
	t.Parallel()
	got := string(RenderDescription("Just a synopsis"))
	want := `<p class="synopsis">Just a synopsis</p>
`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
