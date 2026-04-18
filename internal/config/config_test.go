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

func TestLoad_RejectsHTTPURL(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "http://example.com/repo/"
[metadata]
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for http:// URL")
	}
	if !strings.Contains(err.Error(), "https://") {
		t.Errorf("expected https scheme error, got: %v", err)
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

func TestLoad_RejectsNewlineInMetadata(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "origin",
			body: "[repo]\nname=\"Test\"\nurl=\"https://example.com/repo/\"\n[metadata]\norigin=\"Bad\\nInjected: yes\"\n",
			want: "metadata.origin",
		},
		{
			name: "label",
			body: "[repo]\nname=\"Test\"\nurl=\"https://example.com/repo/\"\n[metadata]\nlabel=\"Bad\\rInjected: yes\"\n",
			want: "metadata.label",
		},
		{
			name: "description",
			body: "[repo]\nname=\"Test\"\nurl=\"https://example.com/repo/\"\n[metadata]\ndescription=\"line1\\nline2\"\n",
			want: "metadata.description",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := writeConfig(t, tc.body)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected newline-injection error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "newline") {
				t.Errorf("expected newline error mentioning %q, got: %v", tc.want, err)
			}
		})
	}
}

func TestLoad_RejectsReservedSuiteNames(t *testing.T) {
	t.Parallel()
	for _, suite := range []string{"pool", "dists", "resources", "templates"} {
		t.Run(suite, func(t *testing.T) {
			t.Parallel()
			path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[metadata]
suites = ["`+suite+`"]
`)
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected error for reserved suite %q", suite)
			}
			if !strings.Contains(err.Error(), "reserved") {
				t.Errorf("expected reserved-suite error, got: %v", err)
			}
		})
	}
}

func TestLoad_HostingDefaults(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[github]
org_name = "TestOrg"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Hosting.Owner != "" {
		t.Errorf("Hosting.Owner should be empty before ResolveHosting, got %q", cfg.Hosting.Owner)
	}
	if cfg.Hosting.TagPrefix != "pool-" {
		t.Errorf("Hosting.TagPrefix = %q, want %q", cfg.Hosting.TagPrefix, "pool-")
	}
}

func TestLoad_HostingExplicit(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[hosting]
owner      = "MyOrg"
repo       = "my-repo"
tag_prefix = "deb-"
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Hosting.Owner != "MyOrg" {
		t.Errorf("Hosting.Owner = %q", cfg.Hosting.Owner)
	}
	if cfg.Hosting.Repo != "my-repo" {
		t.Errorf("Hosting.Repo = %q", cfg.Hosting.Repo)
	}
	if cfg.Hosting.TagPrefix != "deb-" {
		t.Errorf("Hosting.TagPrefix = %q", cfg.Hosting.TagPrefix)
	}
}

func TestLoad_HostingEmptyTagPrefix(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[hosting]
tag_prefix = ""
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Hosting.TagPrefix != "" {
		t.Errorf("Hosting.TagPrefix = %q, want empty", cfg.Hosting.TagPrefix)
	}
}

func TestResolveHosting_DefaultsFromOrgName(t *testing.T) {
	t.Parallel()
	cfg := &RepoConfig{
		OrgName: "TestOrg",
		Hosting: HostingConfig{TagPrefix: "pool-"},
	}
	if err := cfg.ResolveHosting("/fake/root/myrepo"); err != nil {
		t.Fatal(err)
	}
	if cfg.Hosting.Owner != "TestOrg" {
		t.Errorf("Owner = %q, want TestOrg", cfg.Hosting.Owner)
	}
	if cfg.Hosting.Repo != "myrepo" {
		t.Errorf("Repo = %q, want myrepo", cfg.Hosting.Repo)
	}
}

func TestResolveHosting_ExplicitOverridesDefaults(t *testing.T) {
	t.Parallel()
	cfg := &RepoConfig{
		OrgName: "TestOrg",
		Hosting: HostingConfig{Owner: "Other", Repo: "other-repo", TagPrefix: "v-"},
	}
	if err := cfg.ResolveHosting("/fake/root/myrepo"); err != nil {
		t.Fatal(err)
	}
	if cfg.Hosting.Owner != "Other" {
		t.Errorf("Owner = %q, want Other", cfg.Hosting.Owner)
	}
	if cfg.Hosting.Repo != "other-repo" {
		t.Errorf("Repo = %q, want other-repo", cfg.Hosting.Repo)
	}
}

func TestResolveHosting_MissingOwnerErrors(t *testing.T) {
	t.Parallel()
	cfg := &RepoConfig{Hosting: HostingConfig{TagPrefix: "pool-"}}
	err := cfg.ResolveHosting("/fake/root/myrepo")
	if err == nil {
		t.Fatal("expected error when owner cannot be inferred")
	}
	if !strings.Contains(err.Error(), "hosting.owner") || !strings.Contains(err.Error(), "github.org_name") {
		t.Errorf("error should name both fields, got: %v", err)
	}
}

func TestHostingConfig_ReleaseTag(t *testing.T) {
	t.Parallel()
	h := HostingConfig{TagPrefix: "pool-"}
	if got := h.ReleaseTag("stable"); got != "pool-stable" {
		t.Errorf("ReleaseTag = %q", got)
	}
	h2 := HostingConfig{TagPrefix: ""}
	if got := h2.ReleaseTag("beta"); got != "beta" {
		t.Errorf("ReleaseTag with empty prefix = %q", got)
	}
}

func TestHostingConfig_AssetURL(t *testing.T) {
	t.Parallel()
	h := HostingConfig{Owner: "PlayDay-iOS", Repo: "repo", TagPrefix: "pool-"}
	got := h.AssetURL("stable", "test_1.0+beta_iphoneos-arm.deb")
	want := "https://github.com/PlayDay-iOS/repo/releases/download/pool-stable/test_1.0+beta_iphoneos-arm.deb"
	if got != want {
		t.Errorf("AssetURL = %q, want %q", got, want)
	}
}

func TestLoad_InvalidHostingOwner(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[hosting]
owner = "evil/org"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid hosting.owner")
	}
	if !strings.Contains(err.Error(), "hosting.owner") {
		t.Errorf("expected hosting.owner error, got: %v", err)
	}
}

func TestLoad_InvalidHostingRepo(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, `
[repo]
name = "Test"
url  = "https://example.com/repo/"
[hosting]
repo = "../escape"
`)
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for invalid hosting.repo")
	}
	if !strings.Contains(err.Error(), "hosting.repo") {
		t.Errorf("expected hosting.repo error, got: %v", err)
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
