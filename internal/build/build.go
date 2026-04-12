package build

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/deb"
	"github.com/PlayDay-iOS/repo/internal/fileutil"
	"github.com/PlayDay-iOS/repo/internal/page"
	"github.com/PlayDay-iOS/repo/internal/repo"
	"github.com/PlayDay-iOS/repo/internal/textutil"
)

// Options configures the build.
type Options struct {
	RootDir       string
	OutputDir     string
	ConfigPath    string
	TemplatePath  string
	BuildTime     time.Time
	GPGKey        string // armored private key (takes precedence over config gpg_key_file)
	GPGPassphrase string
	Logger        *slog.Logger
}

// ResolveBuildTime returns the build timestamp from the given override,
// or the current time if the override is zero.
func ResolveBuildTime(override time.Time) time.Time {
	if !override.IsZero() {
		return override
	}
	return time.Now().UTC()
}

// Run executes the full repository build.
func Run(opts Options) error {
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}

	cfg, err := config.Load(opts.ConfigPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Load GPG key: Options.GPGKey takes precedence over config file path
	armoredKey := opts.GPGKey
	if armoredKey == "" {
		var keyErr error
		armoredKey, keyErr = loadGPGKey(cfg.GPGKeyFile)
		if keyErr != nil {
			return fmt.Errorf("loading GPG key: %w", keyErr)
		}
	}

	// Clean and create output directory
	absOut, err := filepath.Abs(opts.OutputDir)
	if err != nil {
		return fmt.Errorf("resolving output dir: %w", err)
	}
	if err := validateOutputDir(absOut); err != nil {
		return err
	}

	const outputMarker = ".repotool-output"
	if err := validateExistingOutputDir(absOut, outputMarker); err != nil {
		return err
	}

	if err := os.RemoveAll(absOut); err != nil {
		return fmt.Errorf("cleaning output dir: %w", err)
	}
	opts.OutputDir = absOut

	if err := os.MkdirAll(absOut, 0755); err != nil {
		return fmt.Errorf("creating output dir: %w", err)
	}

	if err := os.WriteFile(filepath.Join(absOut, outputMarker), nil, 0644); err != nil {
		return fmt.Errorf("writing output marker: %w", err)
	}

	buildTime := ResolveBuildTime(opts.BuildTime)

	// Build each suite
	component := cfg.PrimaryComponent()
	allowedArch := cfg.AllowedArchitectures()
	for _, suite := range cfg.Suites {
		if err := buildSuite(opts, cfg, suite, component, allowedArch, armoredKey, opts.GPGPassphrase, buildTime); err != nil {
			return err
		}
	}

	// Copy root icon
	if err := copyRootIcon(opts.RootDir, opts.OutputDir); err != nil {
		return fmt.Errorf("copying root icon: %w", err)
	}

	// Copy public key
	pubKey := filepath.Join(opts.RootDir, "repo-public.key")
	if _, err := os.Stat(pubKey); err == nil {
		if err := fileutil.CopyFile(pubKey, filepath.Join(opts.OutputDir, "repo-public.key")); err != nil {
			return fmt.Errorf("copying public key: %w", err)
		}
	}

	// Render landing page
	if err := page.RenderLandingPage(opts.OutputDir, cfg, opts.TemplatePath, buildTime); err != nil {
		return fmt.Errorf("rendering landing page: %w", err)
	}

	log.Info("repository built", "output", opts.OutputDir)
	return nil
}

func buildSuite(opts Options, cfg *config.RepoConfig, suite, component string, allowedArch map[string]bool, armoredKey, passphrase string, buildTime time.Time) error {
	suiteDir := filepath.Join(opts.OutputDir, suite)
	if err := os.MkdirAll(suiteDir, 0755); err != nil {
		return fmt.Errorf("creating suite dir %s: %w", suite, err)
	}

	// Scan source pool for this suite
	poolSuiteDir := filepath.Join(opts.RootDir, "pool", suite, component)
	poolInfo, statErr := os.Stat(poolSuiteDir)
	if statErr != nil && !os.IsNotExist(statErr) {
		return fmt.Errorf("checking pool dir %s: %w", poolSuiteDir, statErr)
	}

	if statErr != nil || !poolInfo.IsDir() {
		if err := repo.WritePackagesAll(nil, suiteDir); err != nil {
			return fmt.Errorf("writing empty packages for %s: %w", suite, err)
		}
	} else {
		entries, err := deb.ScanPool(opts.RootDir, poolSuiteDir, allowedArch)
		if err != nil {
			return fmt.Errorf("scanning pool for %s: %w", suite, err)
		}
		if err := repo.WritePackagesAll(entries, suiteDir); err != nil {
			return fmt.Errorf("writing packages for %s: %w", suite, err)
		}
	}

	// Copy pool into suite dir for client access
	suitePoolSource := filepath.Join(opts.RootDir, "pool", suite)
	if _, err := os.Stat(suitePoolSource); err == nil {
		suitePoolTarget := filepath.Join(suiteDir, "pool", suite)
		if err := os.MkdirAll(suitePoolTarget, 0755); err != nil {
			return fmt.Errorf("creating pool dir for %s: %w", suite, err)
		}
		if err := fileutil.CopyDir(suitePoolSource, suitePoolTarget); err != nil {
			return fmt.Errorf("copying pool for %s: %w", suite, err)
		}
	}

	iconSrc := filepath.Join(opts.RootDir, "resources", "CydiaIcon.png")
	if _, err := os.Stat(iconSrc); err == nil {
		if err := fileutil.CopyFile(iconSrc, filepath.Join(suiteDir, "CydiaIcon.png")); err != nil {
			return fmt.Errorf("copying suite icon for %s: %w", suite, err)
		}
	}

	// Generate Release
	suiteSuffix := " (" + textutil.TitleCase(suite) + ")"
	withSuffix := func(base string) string {
		if base == "" {
			return ""
		}
		return base + suiteSuffix
	}
	releaseParams := repo.ReleaseParams{
		Origin:        withSuffix(cfg.Origin),
		Label:         withSuffix(cfg.Label),
		Suite:         suite,
		Codename:      suite,
		Architectures: strings.Join(cfg.Architectures, " "),
		Components:    ".",
		Description:   withSuffix(cfg.Description),
		Date:          buildTime,
	}
	if err := repo.WriteRelease(releaseParams, suiteDir); err != nil {
		return fmt.Errorf("writing release for %s: %w", suite, err)
	}

	// Sign
	if err := repo.SignRelease(suiteDir, armoredKey, passphrase); err != nil {
		return fmt.Errorf("signing %s: %w", suite, err)
	}

	// Suite index page
	if err := page.WriteSuiteIndexHTML(suiteDir, suite, cfg.URL); err != nil {
		return fmt.Errorf("writing suite index for %s: %w", suite, err)
	}

	return nil
}

func copyRootIcon(rootDir, outputDir string) error {
	iconSrc := filepath.Join(rootDir, "resources", "CydiaIcon.png")
	if _, err := os.Stat(iconSrc); err != nil {
		return nil // no icon, not an error
	}
	return fileutil.CopyFile(iconSrc, filepath.Join(outputDir, "CydiaIcon.png"))
}

// validateOutputDir rejects paths that would be catastrophic to RemoveAll.
func validateOutputDir(absPath string) error {
	if absPath == "/" || absPath == filepath.Dir(absPath) {
		return fmt.Errorf("refusing to use filesystem root as output dir: %s", absPath)
	}
	blocked := []string{
		"/bin", "/sbin", "/usr", "/lib", "/lib64",
		"/etc", "/var", "/root", "/home",
		"/boot", "/dev", "/proc", "/sys", "/run",
		"/tmp", "/opt", "/srv", "/mnt", "/media", "/snap",
	}
	if slices.Contains(blocked, absPath) {
		return fmt.Errorf("refusing to use system directory as output dir: %s", absPath)
	}
	return nil
}

func validateExistingOutputDir(absPath, marker string) error {
	info, err := os.Lstat(absPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking output path %s: %w", absPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output path %s is a symlink; refusing to delete", absPath)
	}
	if !info.IsDir() {
		return fmt.Errorf("output path %s exists and is not a directory; refusing to delete", absPath)
	}

	markerPath := filepath.Join(absPath, marker)
	markerInfo, err := os.Lstat(markerPath)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("output directory %s exists but has no %s marker — refusing to delete (remove manually or add the marker)", absPath, marker)
		}
		return fmt.Errorf("checking output marker %s: %w", markerPath, err)
	}
	if markerInfo.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("output marker %s is a symlink; refusing to delete", markerPath)
	}
	if !markerInfo.Mode().IsRegular() {
		return fmt.Errorf("output marker %s is not a regular file; refusing to delete", markerPath)
	}

	return nil
}

func loadGPGKey(keyFile string) (string, error) {
	if keyFile != "" {
		data, err := os.ReadFile(keyFile)
		if err != nil {
			return "", fmt.Errorf("reading GPG key file %s: %w", keyFile, err)
		}
		return string(data), nil
	}
	return "", nil
}
