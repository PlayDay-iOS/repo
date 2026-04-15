package depiction

import (
	"log/slog"
	"path"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
)

// entryBaseName derives the unique depiction key from an entry's Filename
// ("pool/stable/main/foo.deb" -> "foo"). PackageEntry.Filename is slash-
// separated (enforced by ScanPool), so path.Base is safe.
func entryBaseName(e *deb.PackageEntry) string {
	return strings.TrimSuffix(path.Base(e.Filename), ".deb")
}

// EnrichEntries mutates each entry's ControlData in place to set Depiction
// and SileoDepiction fields to repotool-hosted URLs. If the original control
// carried a Depiction value and no Homepage, that original value is moved to
// Homepage so it remains discoverable.
//
// The operation is safe to call multiple times on the same slice — the
// second call re-applies the same URLs and never reintroduces a previously
// moved Homepage, because the Homepage-guard depends on the *current*
// Depiction being the maintainer's value, which is no longer true after the
// first call.
func EnrichEntries(entries []*deb.PackageEntry, cfg *config.RepoConfig) {
	for _, e := range entries {
		pkg := e.Control.Get("Package")
		ver := e.Control.Get("Version")
		bn := entryBaseName(e)

		origDepiction := e.Control.Get("Depiction")
		isOurURL := origDepiction == PackageDepictionURL(cfg.URL, bn)

		if existing := e.Control.Get("SileoDepiction"); existing != "" && existing != PackageSileoURL(cfg.URL, bn) {
			slog.Debug("depiction: overwriting existing SileoDepiction", "package", pkg, "version", ver, "existing", existing)
		}

		e.Control.Set("Depiction", PackageDepictionURL(cfg.URL, bn))
		e.Control.Set("SileoDepiction", PackageSileoURL(cfg.URL, bn))

		if origDepiction != "" && !isOurURL && e.Control.Get("Homepage") == "" {
			e.Control.Set("Homepage", origDepiction)
		}
	}
}
