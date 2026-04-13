package ghimport

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PlayDay-iOS/repo/internal/fileutil"
	"github.com/google/go-github/v84/github"
)

// NewGitHubClient creates a GitHub client with optional token auth and custom API base.
func NewGitHubClient(token, apiBase string) (*github.Client, error) {
	var client *github.Client
	if token != "" {
		client = github.NewClient(nil).WithAuthToken(token)
	} else {
		client = github.NewClient(nil)
	}

	if apiBase != "" {
		parsed, err := url.Parse(strings.TrimRight(apiBase, "/") + "/")
		if err != nil {
			return nil, fmt.Errorf("parsing API base URL: %w", err)
		}
		client.BaseURL = parsed
	}

	return client, nil
}

// FetchAllReleases fetches all releases for the given org/repo, paginating as needed.
func FetchAllReleases(ctx context.Context, log *slog.Logger, client *github.Client, org, repo string, perPage int) ([]*github.RepositoryRelease, error) {
	var all []*github.RepositoryRelease
	opts := &github.ListOptions{PerPage: perPage, Page: 1}
	for {
		page, resp, err := client.Repositories.ListReleases(ctx, org, repo, opts)
		if err != nil {
			return nil, fmt.Errorf("listing releases for %s/%s: %w", org, repo, err)
		}
		if resp.Rate.Remaining < 10 {
			log.Warn("GitHub API rate limit low",
				"remaining", resp.Rate.Remaining,
				"reset", resp.Rate.Reset.Time.Format(time.RFC3339))
		}
		all = append(all, page...)
		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}
	return all, nil
}

// MaxDownloadSize caps a single asset download (500 MB).
const MaxDownloadSize = 500 * 1024 * 1024

// DownloadFile downloads a URL to the given path using the provided HTTP client.
// Only HTTPS URLs are accepted; redirects are restricted to HTTPS.
// Downloads are capped at MaxDownloadSize bytes.
func DownloadFile(ctx context.Context, dlURL, dst string, httpClient *http.Client) error {
	if !strings.HasPrefix(dlURL, "https://") {
		return fmt.Errorf("download URL must use HTTPS scheme: %s", dlURL)
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: 60 * time.Second}
	}

	// Copy client to avoid mutating shared CheckRedirect state.
	safeDL := *httpClient
	origRedirect := safeDL.CheckRedirect
	safeDL.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		if req.URL.Scheme != "https" {
			return fmt.Errorf("refusing non-HTTPS redirect to %s", req.URL.Host)
		}
		// Drop auth credentials when redirecting to a different host to
		// prevent leaking API tokens to unrelated servers.
		if len(via) > 0 && req.URL.Host != via[0].URL.Host {
			req.Header.Del("Authorization")
		}
		if origRedirect != nil {
			return origRedirect(req, via)
		}
		return nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dlURL, nil)
	if err != nil {
		return err
	}
	resp, err := safeDL.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// LimitReader is the authoritative size guard; a missing or misreported
	// Content-Length header cannot bypass it.
	return fileutil.WriteAtomic(dst, 0644, func(w io.Writer) error {
		limited := io.LimitReader(resp.Body, MaxDownloadSize+1)
		n, err := io.Copy(w, limited)
		if err != nil {
			return err
		}
		if n > MaxDownloadSize {
			return fmt.Errorf("download exceeded %d byte limit", MaxDownloadSize)
		}
		return nil
	})
}
