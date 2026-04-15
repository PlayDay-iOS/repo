package depiction

import (
	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
)

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

		origDepiction := e.Control.Get("Depiction")
		isOurURL := origDepiction == PackageDepictionURL(cfg.URL, pkg, ver)

		e.Control.Set("Depiction", PackageDepictionURL(cfg.URL, pkg, ver))
		e.Control.Set("SileoDepiction", PackageSileoURL(cfg.URL, pkg, ver))

		if origDepiction != "" && !isOurURL && e.Control.Get("Homepage") == "" {
			e.Control.Set("Homepage", origDepiction)
		}
	}
}
