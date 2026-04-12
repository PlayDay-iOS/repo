package config

import (
    "os"
    "path/filepath"
    "strings"
    "testing"
)

func TestCheckMultiComponentError(t *testing.T) {
    dir := t.TempDir()
    confPath := filepath.Join(dir, "repo.toml")
    os.WriteFile(confPath, []byte(`
[repo]
name = "Test"
url = "https://example.com/repo"
[metadata]
components = ["main", "extras"]
`), 0644)
    _, err := Load(confPath)
    if err == nil {
        t.Fatal("expected error")
    }
    t.Logf("Actual error: %v", err)
    if !strings.Contains(err.Error(), "component") {
        t.Errorf("Error is about something else: %v", err)
    }
}
