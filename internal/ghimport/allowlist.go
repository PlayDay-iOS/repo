package ghimport

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/PlayDay-iOS/repo/internal/validate"
)

// ReadAllowlist parses the allowlist file, returning repo names.
// Blank lines and lines starting with # are ignored.
func ReadAllowlist(log *slog.Logger, path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading allowlist %s: %w", path, err)
	}

	seen := make(map[string]bool)
	var repos []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !validate.Name.MatchString(line) {
			return nil, fmt.Errorf("invalid repo name in allowlist: %q", line)
		}
		if seen[line] {
			log.Warn("duplicate entry in allowlist", "repo", line)
			continue
		}
		seen[line] = true
		repos = append(repos, line)
	}
	return repos, nil
}
