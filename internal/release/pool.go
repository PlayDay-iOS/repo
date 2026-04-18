package release

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
	"github.com/google/go-github/v84/github"
)

// poolEntry tracks a single .deb by its canonical (resolved) path
// and the suite it belongs to.
type poolEntry struct {
	canonicalPath string
	basename      string
	suite         string
	size          int64
}

// PublishPool walks the pool directory, deduplicates by canonical path,
// and uploads missing assets to the appropriate GitHub Release per suite.
func PublishPool(ctx context.Context, client *github.Client, cfg *config.RepoConfig, rootDir string) error {
	entries, err := collectPoolEntries(rootDir, cfg.Suites, cfg.Component)
	if err != nil {
		return fmt.Errorf("collecting pool entries: %w", err)
	}

	// Group by canonical suite tag
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
			slog.Info("no assets for suite", "suite", suite, "tag", tag)
			continue
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
	}

	return nil
}

// collectPoolEntries walks all suite pool dirs, resolves symlinks, and
// deduplicates by canonical path (so a symlink in beta pointing to stable
// only appears once, under stable's suite).
func collectPoolEntries(rootDir string, suites []string, component string) ([]poolEntry, error) {
	seen := make(map[string]bool) // canonical path → already collected
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
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".deb") {
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

			if seen[canonical] {
				return nil
			}
			seen[canonical] = true

			fi, err := os.Stat(canonical)
			if err != nil {
				return fmt.Errorf("stat %s: %w", canonical, err)
			}
			if !fi.Mode().IsRegular() {
				return nil
			}

			// Reuse deb.CanonicalSuite to extract the suite from the resolved path
			canonSuite, err := deb.CanonicalSuite(rootDir, canonical)
			if err != nil {
				return err
			}

			entries = append(entries, poolEntry{
				canonicalPath: canonical,
				basename:      filepath.Base(canonical),
				suite:         canonSuite,
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
