package config

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/validate"
	"github.com/spf13/viper"
)

// HostingConfig holds GitHub Releases hosting settings.
type HostingConfig struct {
	Owner     string
	Repo      string
	TagPrefix string
}

// ReleaseTag returns the GitHub Release tag for a given suite.
func (h HostingConfig) ReleaseTag(suite string) string {
	return h.TagPrefix + suite
}

// AssetURL returns the absolute download URL for a release asset.
func (h HostingConfig) AssetURL(suite, basename string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
		h.Owner, h.Repo, h.ReleaseTag(suite), basename)
}

// RepoConfig holds the parsed repository configuration.
type RepoConfig struct {
	Name          string
	URL           string
	Origin        string
	Label         string
	Suites        []string
	Component     string
	Architectures []string
	Description   string
	OrgName       string
	Hosting       HostingConfig
}

// Load reads and validates the config file at path.
func Load(path string) (*RepoConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetDefault("metadata.suites", []string{"stable"})
	v.SetDefault("metadata.component", "main")
	v.SetDefault("metadata.architectures", []string{"iphoneos-arm", "iphoneos-arm64", "all"})

	if err := v.BindEnv("github.org_name", "ORG_NAME"); err != nil {
		return nil, fmt.Errorf("binding env: %w", err)
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	cfg := &RepoConfig{
		Name:          v.GetString("repo.name"),
		URL:           v.GetString("repo.url"),
		Origin:        v.GetString("metadata.origin"),
		Label:         v.GetString("metadata.label"),
		Suites:        v.GetStringSlice("metadata.suites"),
		Component:     v.GetString("metadata.component"),
		Architectures: v.GetStringSlice("metadata.architectures"),
		Description:   v.GetString("metadata.description"),
		OrgName:       v.GetString("github.org_name"),
	}

	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return nil, fmt.Errorf("repo.name: required")
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return nil, fmt.Errorf("repo.url: required")
	}
	if !strings.HasPrefix(cfg.URL, "https://") {
		return nil, fmt.Errorf("repo.url: must use https:// scheme, got %q", cfg.URL)
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/") + "/"

	if len(cfg.Suites) == 0 {
		return nil, fmt.Errorf("metadata.suites: must not be empty")
	}
	cfg.Component = strings.TrimSpace(cfg.Component)
	if cfg.Component == "" {
		return nil, fmt.Errorf("metadata.component: required")
	}
	if !validate.Name.MatchString(cfg.Component) {
		return nil, fmt.Errorf("metadata.component: invalid %q (must be alphanumeric with .-_ only)", cfg.Component)
	}
	if len(cfg.Architectures) == 0 {
		return nil, fmt.Errorf("metadata.architectures: must not be empty")
	}

	reservedNames := map[string]bool{
		"pool": true, "dists": true, "resources": true, "templates": true,
	}
	seenSuites := make(map[string]bool, len(cfg.Suites))
	for _, s := range cfg.Suites {
		if !validate.Name.MatchString(s) {
			return nil, fmt.Errorf("metadata.suites: invalid suite %q (must be alphanumeric with .-_ only)", s)
		}
		if reservedNames[s] {
			return nil, fmt.Errorf("metadata.suites: suite %q is reserved and would conflict with build output", s)
		}
		if seenSuites[s] {
			return nil, fmt.Errorf("metadata.suites: duplicate suite %q", s)
		}
		seenSuites[s] = true
	}
	for _, a := range cfg.Architectures {
		if !validate.Name.MatchString(a) {
			return nil, fmt.Errorf("metadata.architectures: invalid architecture %q (must be alphanumeric with .-_ only)", a)
		}
	}

	cfg.OrgName = strings.TrimSpace(cfg.OrgName)
	if cfg.OrgName != "" && !validate.Name.MatchString(cfg.OrgName) {
		return nil, fmt.Errorf("github.org_name: invalid %q (must be alphanumeric with .-_ only)", cfg.OrgName)
	}

	// Reject newlines in free-form string fields that end up in Release stanzas.
	for name, val := range map[string]*string{
		"metadata.origin": &cfg.Origin, "metadata.label": &cfg.Label, "metadata.description": &cfg.Description,
	} {
		if strings.ContainsAny(*val, "\n\r") {
			return nil, fmt.Errorf("%s: must not contain newlines", name)
		}
	}

	// Parse [hosting] block with defaults.
	tagPrefix := v.GetString("hosting.tag_prefix")
	if !v.IsSet("hosting.tag_prefix") {
		tagPrefix = "pool-"
	}
	cfg.Hosting = HostingConfig{
		Owner:     strings.TrimSpace(v.GetString("hosting.owner")),
		Repo:      strings.TrimSpace(v.GetString("hosting.repo")),
		TagPrefix: tagPrefix,
	}

	if cfg.Hosting.Owner != "" && !validate.Name.MatchString(cfg.Hosting.Owner) {
		return nil, fmt.Errorf("hosting.owner: invalid %q (must be alphanumeric with .-_ only)", cfg.Hosting.Owner)
	}
	if cfg.Hosting.Repo != "" && !validate.Name.MatchString(cfg.Hosting.Repo) {
		return nil, fmt.Errorf("hosting.repo: invalid %q (must be alphanumeric with .-_ only)", cfg.Hosting.Repo)
	}

	return cfg, nil
}

// ResolveHosting fills in hosting defaults from OrgName and rootDir.
// Must be called before using Hosting.AssetURL.
func (c *RepoConfig) ResolveHosting(rootDir string) error {
	if c.Hosting.Owner == "" {
		c.Hosting.Owner = c.OrgName
	}
	if c.Hosting.Owner == "" {
		return fmt.Errorf("hosting.owner or github.org_name required for release hosting")
	}
	if c.Hosting.Repo == "" {
		c.Hosting.Repo = filepath.Base(rootDir)
	}
	return nil
}

// PrimarySuite returns the first configured suite (guaranteed non-empty after Load).
func (c *RepoConfig) PrimarySuite() string {
	return c.Suites[0]
}

// AllowedArchitectures returns the configured architectures as a lookup set.
func (c *RepoConfig) AllowedArchitectures() map[string]bool {
	m := make(map[string]bool, len(c.Architectures))
	for _, a := range c.Architectures {
		m[a] = true
	}
	return m
}
