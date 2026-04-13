package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeConfig writes a repo.toml with the given body and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "repo.toml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoad_ValidConfig(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test Repo"
url  = "https://example.com/repo"

[metadata]
origin        = "TestOrg"
label         = "TestLabel"
suites        = ["stable", "beta"]
component     = "main"
architectures = ["iphoneos-arm64", "all"]
description   = "Test repo"

[github]
org_name = "TestOrg"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "Test Repo" {
		t.Errorf("Name = %q", cfg.Name)
	}
	if cfg.URL != "https://example.com/repo/" {
		t.Errorf("URL = %q, want trailing slash", cfg.URL)
	}
	if cfg.PrimarySuite() != "stable" {
		t.Errorf("PrimarySuite = %q", cfg.PrimarySuite())
	}
	if cfg.Component != "main" {
		t.Errorf("Component = %q", cfg.Component)
	}
	if cfg.OrgName != "TestOrg" {
		t.Errorf("OrgName = %q", cfg.OrgName)
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if len(cfg.Suites) != 1 || cfg.Suites[0] != "stable" {
		t.Errorf("default Suites = %v", cfg.Suites)
	}
	if cfg.Component != "main" {
		t.Errorf("default Component = %q", cfg.Component)
	}
	if len(cfg.Architectures) != 3 {
		t.Errorf("default Architectures = %v", cfg.Architectures)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
`)

	t.Setenv("ORG_NAME", "EnvOrg")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OrgName != "EnvOrg" {
		t.Errorf("OrgName = %q, want EnvOrg", cfg.OrgName)
	}
}

func TestLoad_EmptyArchitectures(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
architectures = []
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for empty architectures")
	}
}

func TestLoad_EmptyComponentRejected(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
component = ""
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty component")
	}
	if !strings.Contains(err.Error(), "component") {
		t.Errorf("expected component error, got: %v", err)
	}
}

func TestLoad_InvalidComponentName(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
component = "bad name"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid component name")
	}
}

func TestLoad_DuplicateSuitesRejected(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo"
[metadata]
suites = ["stable", "stable"]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for duplicate suites")
	}
	if !strings.Contains(err.Error(), "duplicate suite") {
		t.Errorf("expected duplicate suite error, got: %v", err)
	}
}

func TestLoad_MissingNameErrors(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `[repo]
url = "https://example.com/repo/"
[metadata]
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for missing repo.name")
	}
}

func TestLoad_InvalidArchitectureNameErrors(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
architectures = ["../../etc"]
`)
	if _, err := Load(path); err == nil {
		t.Fatal("expected error for invalid architecture name")
	}
}

func TestLoad_InvalidOrgNameErrors(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[github]
org_name = "../etc"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid org_name")
	}
	if !strings.Contains(err.Error(), "org_name") {
		t.Errorf("expected org_name error, got: %v", err)
	}
}

func TestAllowedArchitectures(t *testing.T) {
	t.Parallel()
	cfg := &RepoConfig{
		Architectures: []string{"iphoneos-arm64", "all"},
	}
	allowed := cfg.AllowedArchitectures()
	if !allowed["iphoneos-arm64"] {
		t.Error("iphoneos-arm64 should be allowed")
	}
	if !allowed["all"] {
		t.Error("'all' should be allowed when in config")
	}
	if allowed["amd64"] {
		t.Error("amd64 should not be allowed")
	}
}
