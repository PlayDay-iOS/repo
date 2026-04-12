package config

import (
	"fmt"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/validate"
	"github.com/spf13/viper"
)

type RepoConfig struct {
	Name          string
	URL           string
	Origin        string
	Label         string
	Suites        []string
	Components    []string
	Architectures []string
	Description   string
	GPGKeyFile    string
	OrgName       string
}

func Load(path string) (*RepoConfig, error) {
	v := viper.New()
	v.SetConfigFile(path)

	// Defaults
	v.SetDefault("metadata.suites", []string{"stable"})
	v.SetDefault("metadata.components", []string{"main"})
	v.SetDefault("metadata.architectures", []string{"iphoneos-arm", "iphoneos-arm64", "all"})

	// Environment overrides
	if err := v.BindEnv("signing.gpg_key_file", "GPG_KEY_FILE"); err != nil {
		return nil, fmt.Errorf("binding env: %w", err)
	}
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
		Components:    v.GetStringSlice("metadata.components"),
		Architectures: v.GetStringSlice("metadata.architectures"),
		Description:   v.GetString("metadata.description"),
		GPGKeyFile:    v.GetString("signing.gpg_key_file"),
		OrgName:       v.GetString("github.org_name"),
	}

	cfg.Name = strings.TrimSpace(cfg.Name)
	if cfg.Name == "" {
		return nil, fmt.Errorf("repo.name is required")
	}
	cfg.URL = strings.TrimSpace(cfg.URL)
	if cfg.URL == "" {
		return nil, fmt.Errorf("repo.url is required")
	}
	if !strings.HasPrefix(cfg.URL, "https://") && !strings.HasPrefix(cfg.URL, "http://") {
		return nil, fmt.Errorf("repo.url must use http:// or https:// scheme, got %q", cfg.URL)
	}
	cfg.URL = strings.TrimRight(cfg.URL, "/") + "/"
	if len(cfg.Suites) == 0 {
		return nil, fmt.Errorf("metadata.suites must not be empty")
	}
	if len(cfg.Components) == 0 {
		return nil, fmt.Errorf("metadata.components must not be empty")
	}
	if len(cfg.Components) != 1 {
		return nil, fmt.Errorf("metadata.components must contain exactly one component, got %d", len(cfg.Components))
	}
	if len(cfg.Architectures) == 0 {
		return nil, fmt.Errorf("metadata.architectures must not be empty")
	}

	reservedNames := map[string]bool{
		"pool": true, "dists": true, "resources": true, "templates": true,
	}
	for _, s := range cfg.Suites {
		if !validate.Name.MatchString(s) {
			return nil, fmt.Errorf("invalid suite name %q: must be alphanumeric with .-_ only", s)
		}
		if reservedNames[s] {
			return nil, fmt.Errorf("suite name %q is reserved and would conflict with build output", s)
		}
	}
	for _, c := range cfg.Components {
		if !validate.Name.MatchString(c) {
			return nil, fmt.Errorf("invalid component name %q: must be alphanumeric with .-_ only", c)
		}
	}
	for _, a := range cfg.Architectures {
		if !validate.Name.MatchString(a) {
			return nil, fmt.Errorf("invalid architecture name %q: must be alphanumeric with .-_ only", a)
		}
	}

	// Sanitize string fields that end up in Release stanzas — reject newlines
	for name, val := range map[string]*string{
		"origin": &cfg.Origin, "label": &cfg.Label, "description": &cfg.Description,
	} {
		if strings.ContainsAny(*val, "\n\r") {
			return nil, fmt.Errorf("config field %q must not contain newlines", name)
		}
	}

	return cfg, nil
}

func (c *RepoConfig) PrimarySuite() string {
	return c.Suites[0]
}

func (c *RepoConfig) PrimaryComponent() string {
	return c.Components[0]
}

func (c *RepoConfig) AllowedArchitectures() map[string]bool {
	m := make(map[string]bool, len(c.Architectures))
	for _, a := range c.Architectures {
		m[a] = true
	}
	return m
}
