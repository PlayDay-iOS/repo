package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "repo.toml")
	if err := os.WriteFile(confPath, []byte(`
[repo]
name = "Test Repo"
url  = "https://example.com/repo"

[metadata]
origin = "TestOrg"
label  = "TestLabel"
suites = ["stable", "beta"]
components = ["main"]
architectures = ["iphoneos-arm64", "all"]
description = "Test repo"

[signing]
gpg_key_file = "/path/to/key.asc"
`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(confPath)
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
	if cfg.PrimaryComponent() != "main" {
		t.Errorf("PrimaryComponent = %q", cfg.PrimaryComponent())
	}
	if cfg.GPGKeyFile != "/path/to/key.asc" {
		t.Errorf("GPGKeyFile = %q", cfg.GPGKeyFile)
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "repo.toml")
	if err := os.WriteFile(confPath, []byte("[repo]\n[metadata]\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(confPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Name != "PlayDay iOS Repo" {
		t.Errorf("default Name = %q", cfg.Name)
	}
	if len(cfg.Suites) != 1 || cfg.Suites[0] != "stable" {
		t.Errorf("default Suites = %v", cfg.Suites)
	}
	if len(cfg.Architectures) != 3 {
		t.Errorf("default Architectures = %v", cfg.Architectures)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "repo.toml")
	if err := os.WriteFile(confPath, []byte("[repo]\n[metadata]\n[signing]\ngpg_key_file = \"from-file\"\n"), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GPG_KEY_FILE", "/env/key.asc")

	cfg, err := Load(confPath)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GPGKeyFile != "/env/key.asc" {
		t.Errorf("GPGKeyFile = %q, expected '/env/key.asc'", cfg.GPGKeyFile)
	}
}

func TestLoad_EmptyArchitectures(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "repo.toml")
	if err := os.WriteFile(confPath, []byte("[repo]\n[metadata]\narchitectures = []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(confPath)
	if err == nil {
		t.Fatal("expected error for empty architectures")
	}
}

func TestLoad_EmptyComponents(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "repo.toml")
	if err := os.WriteFile(confPath, []byte("[repo]\n[metadata]\ncomponents = []\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(confPath)
	if err == nil {
		t.Fatal("expected error for empty components")
	}
}

func TestLoad_MultipleComponentsRejected(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "repo.toml")
	if err := os.WriteFile(confPath, []byte(`
[repo]
url = "https://example.com/repo"
[metadata]
components = ["main", "extras"]
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Load(confPath)
	if err == nil {
		t.Fatal("expected error for multiple components")
	}
}

func TestAllowedArchitectures(t *testing.T) {
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
