package release

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/google/go-github/v84/github"
)

// PublishIndex uploads a built APT index to the release its suite is served
// from. Index files are rewritten on every build, so existing assets are always
// replaced rather than compared.
func PublishIndex(ctx context.Context, client *github.Client, cfg *config.RepoConfig, indexDir string) error {
	pub := NewPublisher(client, cfg.Hosting.Owner, cfg.Hosting.Repo)

	for _, suite := range cfg.Suites {
		suiteDir := filepath.Join(indexDir, suite)
		names, err := indexFiles(suiteDir)
		if err != nil {
			return err
		}
		if len(names) == 0 {
			slog.Info("no index files for suite", "suite", suite, "dir", suiteDir)
			continue
		}

		tag := cfg.Hosting.ReleaseTag(suite)
		releaseID, err := pub.EnsureRelease(ctx, tag)
		if err != nil {
			return err
		}

		existing, err := pub.ListAssets(ctx, releaseID)
		if err != nil {
			return err
		}

		for _, name := range names {
			if err := ctx.Err(); err != nil {
				return err
			}

			path := filepath.Join(suiteDir, name)
			if a, ok := existing[name]; ok {
				slog.Info("replacing index asset", "name", name, "tag", tag)
				if err := pub.ReplaceAsset(ctx, releaseID, a.ID, name, path); err != nil {
					return fmt.Errorf("replacing %s in %s: %w", name, tag, err)
				}
				continue
			}

			slog.Info("uploading index asset", "name", name, "tag", tag)
			if err := pub.UploadAsset(ctx, releaseID, name, path); err != nil {
				return fmt.Errorf("uploading %s to %s: %w", name, tag, err)
			}
		}
	}

	return nil
}

// indexFiles lists the regular files directly inside a built suite directory.
func indexFiles(suiteDir string) ([]string, error) {
	dirEntries, err := os.ReadDir(suiteDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading index dir %s: %w", suiteDir, err)
	}

	var names []string
	for _, d := range dirEntries {
		if !d.Type().IsRegular() {
			continue
		}
		names = append(names, d.Name())
	}
	return names, nil
}
