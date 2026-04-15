package depiction

import (
	"slices"
	"strings"
	"testing"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
)

func makeEntry(fields map[string]string) *deb.PackageEntry {
	keys := make([]string, 0, len(fields))
	// Deterministic order: sort so tests are stable.
	for k := range fields {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return &deb.PackageEntry{Control: deb.NewControlData(keys, fields)}
}

func TestEnrichEntries_InjectsDepictionAndSileoFields(t *testing.T) {
	t.Parallel()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	e := makeEntry(map[string]string{
		"Package": "com.foo.bar",
		"Version": "1.0",
	})

	EnrichEntries([]*deb.PackageEntry{e}, cfg)

	if got := e.Control.Get("Depiction"); got != "https://example.com/repo/depictions/com.foo.bar/1.0/depiction.html" {
		t.Errorf("Depiction = %q", got)
	}
	if got := e.Control.Get("SileoDepiction"); got != "https://example.com/repo/depictions/com.foo.bar/1.0/sileo.json" {
		t.Errorf("SileoDepiction = %q", got)
	}
}

func TestEnrichEntries_PreservesOriginalDepictionAsHomepageWhenHomepageAbsent(t *testing.T) {
	t.Parallel()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	e := makeEntry(map[string]string{
		"Package":   "com.foo.bar",
		"Version":   "1.0",
		"Depiction": "https://original.example/depiction.html",
	})

	EnrichEntries([]*deb.PackageEntry{e}, cfg)

	if got := e.Control.Get("Homepage"); got != "https://original.example/depiction.html" {
		t.Errorf("Homepage = %q, want original depiction URL", got)
	}
}

func TestEnrichEntries_DoesNotOverwriteExistingHomepage(t *testing.T) {
	t.Parallel()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	e := makeEntry(map[string]string{
		"Package":   "com.foo.bar",
		"Version":   "1.0",
		"Depiction": "https://original.example/depiction.html",
		"Homepage":  "https://maintainer.example/",
	})

	EnrichEntries([]*deb.PackageEntry{e}, cfg)

	if got := e.Control.Get("Homepage"); got != "https://maintainer.example/" {
		t.Errorf("Homepage = %q, should be untouched", got)
	}
}

func TestEnrichEntries_SkipsHomepageWhenOriginalDepictionAbsent(t *testing.T) {
	t.Parallel()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	e := makeEntry(map[string]string{
		"Package": "com.foo.bar",
		"Version": "1.0",
	})

	EnrichEntries([]*deb.PackageEntry{e}, cfg)

	if got := e.Control.Get("Homepage"); got != "" {
		t.Errorf("Homepage = %q, want empty", got)
	}
}

func TestEnrichEntries_IsIdempotent(t *testing.T) {
	t.Parallel()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	e := makeEntry(map[string]string{
		"Package":   "com.foo.bar",
		"Version":   "1.0",
		"Depiction": "https://original.example/depiction.html",
	})

	EnrichEntries([]*deb.PackageEntry{e}, cfg)
	firstOrder := append([]string(nil), e.Control.Order()...)
	firstStanza := e.Stanza()

	// Second call: Depiction is now the injected URL, not the original.
	// The rule is evaluated against the *current* state, so Homepage should
	// not change on the second invocation.
	EnrichEntries([]*deb.PackageEntry{e}, cfg)
	if !slices.Equal(firstOrder, e.Control.Order()) {
		t.Errorf("Order changed after second enrich: %v -> %v", firstOrder, e.Control.Order())
	}
	if firstStanza != e.Stanza() {
		t.Errorf("Stanza drift after second enrich")
	}
}

func TestEnrichEntries_StanzaRetainsPreExistingKeysInOriginalOrder(t *testing.T) {
	t.Parallel()
	cfg := &config.RepoConfig{URL: "https://example.com/repo/"}
	e := makeEntry(map[string]string{
		"Package":     "com.foo.bar",
		"Version":     "1.0",
		"Maintainer":  "x",
		"Description": "y",
	})
	// Pre-existing order from makeEntry is alphabetic. Verify that after
	// enrichment the injected keys land at the end, preserving the initial
	// prefix.
	EnrichEntries([]*deb.PackageEntry{e}, cfg)

	order := e.Control.Order()
	prefix := order[:4]
	if !slices.Equal(prefix, []string{"Description", "Maintainer", "Package", "Version"}) {
		t.Errorf("original prefix mutated: %v", prefix)
	}
	injected := order[4:]
	// Depiction and SileoDepiction always appended (Homepage only when
	// original Depiction existed, which it didn't here).
	if !slices.Contains(injected, "Depiction") || !slices.Contains(injected, "SileoDepiction") {
		t.Errorf("injected keys missing: %v", injected)
	}
	if strings.Contains(strings.Join(injected, ","), "Homepage") {
		t.Errorf("Homepage unexpectedly injected: %v", injected)
	}
}
