package release

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/v84/github"
)

// Asset holds the metadata we care about for an existing release asset.
type Asset struct {
	ID   int64
	Size int
}

// Publisher manages GitHub Release assets for the deb pool.
type Publisher struct {
	client *github.Client
	owner  string
	repo   string
}

// NewPublisher creates a Publisher for the given owner/repo.
func NewPublisher(client *github.Client, owner, repo string) *Publisher {
	return &Publisher{client: client, owner: owner, repo: repo}
}

// sleepFn is the function used for retry backoff; overridable in tests.
var sleepFn = time.Sleep

// retryable returns true for HTTP status codes that warrant a retry.
func retryable(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// EnsureRelease returns the release ID for the given tag, creating the
// release if it does not exist. Retries on 5xx and 429 up to 3 times.
func (p *Publisher) EnsureRelease(ctx context.Context, tag string) (int64, error) {
	var lastErr error
	for attempt := range 3 {
		rel, resp, err := p.client.Repositories.GetReleaseByTag(ctx, p.owner, p.repo, tag)
		if err == nil {
			return rel.GetID(), nil
		}
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return p.createRelease(ctx, tag)
		}
		if resp != nil && retryable(resp.StatusCode) {
			lastErr = err
			slog.Warn("GitHub API error, retrying", "attempt", attempt+1, "status", resp.StatusCode, "tag", tag)
			sleepFn(time.Duration(1<<uint(attempt)) * time.Second)
			continue
		}
		return 0, fmt.Errorf("getting release %s: %w", tag, err)
	}
	return 0, fmt.Errorf("getting release %s after retries: %w", tag, lastErr)
}

func (p *Publisher) createRelease(ctx context.Context, tag string) (int64, error) {
	rel, _, err := p.client.Repositories.CreateRelease(ctx, p.owner, p.repo, &github.RepositoryRelease{
		TagName: github.Ptr(tag),
		Name:    github.Ptr(tag),
	})
	if err != nil {
		return 0, fmt.Errorf("creating release %s: %w", tag, err)
	}
	return rel.GetID(), nil
}

// ListAssets returns all assets for a release, keyed by asset name.
func (p *Publisher) ListAssets(ctx context.Context, releaseID int64) (map[string]Asset, error) {
	assets := make(map[string]Asset)
	opts := &github.ListOptions{PerPage: 100, Page: 1}
	for {
		page, resp, err := p.client.Repositories.ListReleaseAssets(ctx, p.owner, p.repo, releaseID, opts)
		if err != nil {
			return nil, fmt.Errorf("listing assets for release %d: %w", releaseID, err)
		}
		for _, a := range page {
			assets[a.GetName()] = Asset{ID: a.GetID(), Size: a.GetSize()}
		}
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return assets, nil
}

// UploadAsset uploads a file as a release asset.
func (p *Publisher) UploadAsset(ctx context.Context, releaseID int64, name, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()

	_, _, err = p.client.Repositories.UploadReleaseAsset(ctx, p.owner, p.repo, releaseID, &github.UploadOptions{
		Name: name,
	}, f)
	if err != nil {
		return fmt.Errorf("uploading %s: %w", name, err)
	}
	return nil
}

// ReplaceAsset deletes an existing asset and uploads a replacement.
func (p *Publisher) ReplaceAsset(ctx context.Context, releaseID, oldAssetID int64, name, path string) error {
	if _, err := p.client.Repositories.DeleteReleaseAsset(ctx, p.owner, p.repo, oldAssetID); err != nil {
		return fmt.Errorf("deleting old asset %s (id=%d): %w", name, oldAssetID, err)
	}
	return p.UploadAsset(ctx, releaseID, name, path)
}
