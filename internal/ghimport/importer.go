package ghimport

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
	"github.com/PlayDay-iOS/repo/internal/fileutil"
	"github.com/PlayDay-iOS/repo/internal/hashutil"
	"github.com/google/go-github/v84/github"
)

// Options configures the import command.
type Options struct {
	RootDir            string
	ConfigPath         string
	AllowlistPath      string
	Suite              string
	IncludePrereleases bool
	Token              string // GitHub API token (GH_TOKEN or GITHUB_TOKEN)
	APIBase            string // GitHub API base URL (default: https://api.github.com)
	Logger             *slog.Logger
}

// filePlacer holds the functions used for placing files into the pool.
type filePlacer struct {
	link          func(oldname, newname string) error
	copyExclusive func(src, dst string) error
}

var defaultPlacer = filePlacer{
	link:          os.Link,
	copyExclusive: fileutil.CopyFileExclusive,
}

// Run executes the import process.
func Run(ctx context.Context, opts Options) error {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if cfg.OrgName == "" {
		return fmt.Errorf("repo.org_name is required for import (set in repo.toml or ORG_NAME env)")
	}
	if opts.Token == "" {
		return fmt.Errorf("GitHub API token required for import: set GH_TOKEN or GITHUB_TOKEN (unauthenticated access is rate-limited to 60 req/hr)")
	}

	suite := opts.Suite
	if suite == "" {
		suite = cfg.PrimarySuite()
	}
	if !slices.Contains(cfg.Suites, suite) {
		return fmt.Errorf("suite %q not in configured suites: %v", suite, cfg.Suites)
	}

	destDir := filepath.Join(opts.RootDir, "pool", suite, cfg.Component)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("creating destination dir: %w", err)
	}

	repos, err := ReadAllowlist(log, opts.AllowlistPath)
	if err != nil {
		return err
	}
	if len(repos) == 0 {
		log.Info("allowlist is empty, nothing to import")
		return nil
	}

	perPage := 100
	client, err := NewGitHubClient(opts.Token, opts.APIBase)
	if err != nil {
		return fmt.Errorf("creating GitHub client: %w", err)
	}

	dlClient := &http.Client{
		Transport: client.Client().Transport,
		Timeout:   60 * time.Second,
	}

	allowedArch := cfg.AllowedArchitectures()

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	tmpDir, err := os.MkdirTemp("", "ghimport-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	var added, skipped, rejected int

	for _, repoName := range repos {
		if err := runCtx.Err(); err != nil {
			return err
		}
		log.Info("scanning repo", "org", cfg.OrgName, "repo", repoName)

		releases, err := FetchAllReleases(runCtx, log, client, cfg.OrgName, repoName, perPage)
		if err != nil {
			log.Warn("failed to fetch releases, skipping", "repo", repoName, "error", err)
			skipped++
			continue
		}

		assets := collectDebAssets(releases, opts.IncludePrereleases)
		if len(assets) == 0 {
			log.Info("no matching .deb assets found", "repo", repoName)
			continue
		}

		safeRepo := sanitizeFilename(repoName)
		for _, a := range assets {
			result := processAsset(runCtx, log, a, safeRepo, tmpDir, destDir, allowedArch, dlClient, defaultPlacer)
			switch result {
			case assetAdded:
				added++
			case assetSkipped:
				skipped++
			case assetRejected:
				rejected++
			}
		}
	}

	log.Info("import complete", "suite", suite, "added", added, "skipped", skipped, "rejected", rejected)
	return nil
}

type assetResult int

const (
	assetAdded assetResult = iota
	assetSkipped
	assetRejected
)

func processAsset(ctx context.Context, log *slog.Logger, a debAsset, safeRepo, tmpDir, destDir string, allowedArch map[string]bool, dlClient *http.Client, placer filePlacer) assetResult {
	safeName := sanitizeFilename(a.name)
	tmpPath := filepath.Join(tmpDir, fmt.Sprintf("%s_%s", safeRepo, safeName))
	defer os.Remove(tmpPath)

	log.Info("downloading asset", "name", a.name, "tag", a.tagName)
	if err := DownloadFile(ctx, a.downloadURL, tmpPath, dlClient); err != nil {
		log.Warn("failed to download, skipping", "name", a.name, "error", err)
		return assetSkipped
	}

	debFile, err := os.Open(tmpPath)
	if err != nil {
		log.Warn("failed to open downloaded file, skipping", "name", a.name, "error", err)
		return assetSkipped
	}

	control, err := deb.ExtractControlFromReader(debFile, a.name)
	debFile.Close()
	if err != nil {
		log.Warn("rejected: failed to parse .deb control", "name", a.name, "error", err)
		return assetRejected
	}

	if err := deb.ValidateControl(control, allowedArch); err != nil {
		log.Warn("rejected: invalid control metadata", "name", a.name, "error", err)
		return assetRejected
	}

	arch := control.Get("Architecture")
	pkg := control.Get("Package")
	ver := control.Get("Version")

	if containsPathChars(pkg) || containsPathChars(ver) || containsPathChars(arch) {
		log.Warn("rejected: control fields contain invalid path characters", "name", a.name)
		return assetRejected
	}

	canonicalName := fmt.Sprintf("%s_%s_%s.deb", pkg, ver, arch)
	destPath := filepath.Join(destDir, canonicalName)

	cleanDest := filepath.Clean(destPath)
	if !strings.HasPrefix(cleanDest, filepath.Clean(destDir)+string(os.PathSeparator)) {
		log.Warn("rejected: path traversal detected in control fields", "name", a.name)
		return assetRejected
	}

	return placeFile(log, tmpPath, destPath, canonicalName, placer)
}

func placeFile(log *slog.Logger, tmpPath, destPath, canonicalName string, placer filePlacer) assetResult {
	// Use os.Link for atomic "create only if not exists" semantics,
	// avoiding a TOCTOU race between stat and rename.
	linkErr := placer.link(tmpPath, destPath)
	if linkErr == nil {
		log.Info("added package", "name", canonicalName)
		return assetAdded
	}

	if os.IsExist(linkErr) {
		existingHash, err := hashutil.SHA256File(destPath)
		if err != nil {
			log.Warn("failed to read existing file, skipping", "name", canonicalName, "error", err)
			return assetSkipped
		}
		incomingHash, err := hashutil.SHA256File(tmpPath)
		if err != nil {
			log.Warn("failed to hash incoming file, skipping", "name", canonicalName, "error", err)
			return assetSkipped
		}

		if existingHash == incomingHash {
			log.Debug("unchanged", "name", canonicalName)
			return assetSkipped
		}
		log.Warn("rejected: already exists with different content", "name", canonicalName)
		return assetRejected
	}

	if err := placer.copyExclusive(tmpPath, destPath); err != nil {
		if os.IsExist(err) {
			log.Debug("already placed by concurrent process", "name", canonicalName)
		} else {
			log.Warn("failed to place file, skipping", "name", canonicalName, "error", err)
		}
		return assetSkipped
	}
	log.Info("added package", "name", canonicalName)
	return assetAdded
}

// debAsset identifies a single release asset that is a candidate for import.
type debAsset struct {
	name        string // asset filename as published on GitHub, e.g. "tweak_1.0_arm64.deb"
	downloadURL string // HTTPS URL to the asset
	tagName     string // release tag name, for logging only
}

func collectDebAssets(releases []*github.RepositoryRelease, includePrereleases bool) []debAsset {
	var assets []debAsset
	for _, r := range releases {
		if r.GetDraft() {
			continue
		}
		if !includePrereleases && r.GetPrerelease() {
			continue
		}
		for _, a := range r.Assets {
			name := a.GetName()
			dlURL := a.GetBrowserDownloadURL()
			if !strings.HasSuffix(strings.ToLower(name), ".deb") || dlURL == "" {
				continue
			}
			assets = append(assets, debAsset{
				name:        name,
				downloadURL: dlURL,
				tagName:     r.GetTagName(),
			})
		}
	}
	return assets
}

func sanitizeFilename(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	result := b.String()
	if result == "" {
		return "unknown"
	}
	return result
}

func containsPathChars(s string) bool {
	return strings.ContainsAny(s, "/\\\x00")
}
