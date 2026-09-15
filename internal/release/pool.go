package release

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/google/go-github/v84/github"
)

// poolEntry tracks a single .deb by its canonical (resolved) path, the suite
// it is published under, and the asset name it is uploaded as.
type poolEntry struct {
	canonicalPath string
	basename      string
	suite         string
	size          int64
}

// PublishPool walks the pool directory and uploads missing assets to each
// suite's GitHub Release.
func PublishPool(ctx context.Context, client *github.Client, cfg *config.RepoConfig, rootDir string) error {
	entries, err := collectPoolEntries(rootDir, cfg.Suites, cfg.Component)
	if err != nil {
		return fmt.Errorf("collecting pool entries: %w", err)
	}

	bySuite := make(map[string][]poolEntry)
	for _, e := range entries {
		tag := cfg.Hosting.ReleaseTag(e.suite)
		bySuite[tag] = append(bySuite[tag], e)
	}

	pub := NewPublisher(client, cfg.Hosting.Owner, cfg.Hosting.Repo)

	for _, suite := range cfg.Suites {
		tag := cfg.Hosting.ReleaseTag(suite)
		group := bySuite[tag]
		if len(group) == 0 {
			// Not skipped: a suite emptied of packages still has to lose the
			// payloads a previous run left on its release.
			slog.Info("no assets for suite", "suite", suite, "tag", tag)
		}

		releaseID, err := pub.EnsureRelease(ctx, tag)
		if err != nil {
			return err
		}

		existing, err := pub.ListAssets(ctx, releaseID)
		if err != nil {
			return err
		}

		for _, e := range group {
			if err := ctx.Err(); err != nil {
				return err
			}

			if a, ok := existing[e.basename]; ok {
				if int64(a.Size) == e.size {
					slog.Debug("asset exists, skipping", "name", e.basename, "tag", tag)
					continue
				}
				slog.Warn("asset size mismatch, replacing", "name", e.basename, "tag", tag,
					"local", e.size, "remote", a.Size)
				if err := pub.ReplaceAsset(ctx, releaseID, a.ID, e.basename, e.canonicalPath); err != nil {
					return fmt.Errorf("replacing %s: %w", e.basename, err)
				}
				continue
			}

			slog.Info("uploading", "name", e.basename, "tag", tag)
			if err := pub.UploadAsset(ctx, releaseID, e.basename, e.canonicalPath); err != nil {
				return fmt.Errorf("uploading %s: %w", e.basename, err)
			}
		}

		if err := pruneStaleDebs(ctx, pub, tag, group, existing); err != nil {
			return err
		}
	}

	return nil
}

// pruneStaleDebs deletes payloads left on a suite's release by an earlier run
// that its pool no longer contains. Only .deb assets are considered, which is
// what keeps the index published to the same release (Release, Packages*,
// InRelease, CydiaIcon.png) out of reach.
func pruneStaleDebs(ctx context.Context, pub *Publisher, tag string, group []poolEntry, existing map[string]Asset) error {
	expected := make(map[string]bool, len(group))
	for _, e := range group {
		expected[e.basename] = true
	}

	for _, name := range slices.Sorted(maps.Keys(existing)) {
		if err := ctx.Err(); err != nil {
			return err
		}
		if expected[name] || !isDeb(name) {
			continue
		}
		slog.Warn("pruning stale asset", "name", name, "tag", tag)
		if err := pub.DeleteAsset(ctx, existing[name].ID, name); err != nil {
			return fmt.Errorf("pruning %s from %s: %w", name, tag, err)
		}
	}

	return nil
}

// isDeb reports whether an asset or file name is a Debian package.
func isDeb(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".deb")
}

// collectPoolEntries walks all suite pool dirs and resolves symlinks. Each
// suite's index addresses that suite's own release, so an entry is collected
// per suite it appears in; only a repeated asset name within one suite is
// dropped, since a release tag is a single flat namespace.
func collectPoolEntries(rootDir string, suites []string, component string) ([]poolEntry, error) {
	seen := make(map[string]bool) // suite + asset name → already collected
	var entries []poolEntry

	for _, suite := range suites {
		poolDir := filepath.Join(rootDir, "pool", suite, component)
		if _, err := os.Stat(poolDir); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(poolDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !isDeb(d.Name()) {
				return nil
			}

			// Resolve symlinks
			canonical, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("resolving %s: %w", path, err)
			}
			canonical = filepath.Clean(canonical)

			// Reject symlinks that resolve outside the repo root.
			if !strings.HasPrefix(canonical, rootDir+string(os.PathSeparator)) {
				return fmt.Errorf("resolved path %s escapes root %s", canonical, rootDir)
			}

			basename := d.Name()
			key := suite + "/" + basename
			if seen[key] {
				return nil
			}
			seen[key] = true

			fi, err := os.Stat(canonical)
			if err != nil {
				return fmt.Errorf("stat %s: %w", canonical, err)
			}
			if !fi.Mode().IsRegular() {
				return nil
			}

			entries = append(entries, poolEntry{
				canonicalPath: canonical,
				basename:      basename,
				suite:         suite,
				size:          fi.Size(),
			})
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walking pool/%s/%s: %w", suite, component, err)
		}
	}

	return entries, nil
}
