package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/PlayDay-iOS/repo/internal/build"
	"github.com/PlayDay-iOS/repo/internal/config"
	"github.com/PlayDay-iOS/repo/internal/ghimport"
	"github.com/PlayDay-iOS/repo/internal/page"
)

var version = func() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}()

var configPath string

var rootCmd = &cobra.Command{
	Use:     "repotool",
	Short:   "iOS APT repository builder",
	Version: version,
}

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build repository metadata and landing page",
	RunE:  runBuild,
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import .deb packages from GitHub org releases",
	RunE:  runImport,
}

var renderCmd = &cobra.Command{
	Use:   "render",
	Short: "Render landing page only",
	RunE:  runRender,
}

var (
	flagOutput        string
	flagTemplate      string
	flagAllowlist     string
	flagSuite         string
	flagPrereleases   bool
	flagImportTimeout time.Duration
)

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to repo.toml (default: <cwd>/repo.toml)")

	for _, c := range []*cobra.Command{buildCmd, renderCmd} {
		c.Flags().StringVar(&flagOutput, "output", "", "Output directory (default: <cwd>/_site)")
		c.Flags().StringVar(&flagTemplate, "template", "", "Path to HTML template (default: <cwd>/templates/index.html.tmpl)")
	}

	importCmd.Flags().StringVar(&flagAllowlist, "allowlist", "", "Path to allowlist file (default: <cwd>/org-import-allowlist.txt)")
	importCmd.Flags().StringVar(&flagSuite, "suite", "", "Target suite (default: first entry of metadata.suites, or TARGET_SUITE env)")
	importCmd.Flags().BoolVar(&flagPrereleases, "include-prereleases", false, "Include prerelease assets")
	importCmd.Flags().DurationVar(&flagImportTimeout, "timeout", 0, "Upper bound for the import run (e.g. 30m, 2h); 0 = use built-in default")

	rootCmd.AddCommand(buildCmd, importCmd, renderCmd)
}

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		os.Exit(1)
	}
}

// ghToken returns the GitHub token from GH_TOKEN or GITHUB_TOKEN.
func ghToken() string {
	if v := os.Getenv("GH_TOKEN"); v != "" {
		return v
	}
	return os.Getenv("GITHUB_TOKEN")
}

func runBuild(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	output := flagOutput
	if output == "" {
		output = filepath.Join(root, "_site")
	}
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(root, "repo.toml")
	}
	tmplPath := flagTemplate
	if tmplPath == "" {
		tmplPath = filepath.Join(root, "templates", "index.html.tmpl")
	}

	buildTime, err := build.BuildTimeFromEnv()
	if err != nil {
		return err
	}

	return build.Run(cmd.Context(), build.Options{
		RootDir:       root,
		OutputDir:     output,
		ConfigPath:    cfgPath,
		TemplatePath:  tmplPath,
		BuildTime:     buildTime,
		GPGKey:        os.Getenv("GPG_PRIVATE_KEY"),
		GPGPassphrase: os.Getenv("GPG_PASSPHRASE"),
	})
}

func runImport(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(root, "repo.toml")
	}
	allowlist := flagAllowlist
	if allowlist == "" {
		allowlist = filepath.Join(root, "org-import-allowlist.txt")
	}

	suite := flagSuite
	if suite == "" {
		suite = os.Getenv("TARGET_SUITE")
	}

	prereleases := flagPrereleases
	if !cmd.Flags().Changed("include-prereleases") {
		if envVal := os.Getenv("INCLUDE_PRERELEASES"); envVal != "" {
			if v, parseErr := strconv.ParseBool(envVal); parseErr == nil {
				prereleases = v
			}
		}
	}

	timeout := flagImportTimeout
	if !cmd.Flags().Changed("timeout") {
		if envVal := os.Getenv("IMPORT_TIMEOUT"); envVal != "" {
			d, parseErr := time.ParseDuration(envVal)
			if parseErr != nil {
				return fmt.Errorf("invalid IMPORT_TIMEOUT %q: %w", envVal, parseErr)
			}
			timeout = d
		}
	}

	return ghimport.Run(cmd.Context(), ghimport.Options{
		RootDir:            root,
		ConfigPath:         cfgPath,
		AllowlistPath:      allowlist,
		Suite:              suite,
		IncludePrereleases: prereleases,
		Token:              ghToken(),
		APIBase:            os.Getenv("GITHUB_API_BASE"),
		Timeout:            timeout,
	})
}

func runRender(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	output := flagOutput
	if output == "" {
		output = filepath.Join(root, "_site")
	}
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(root, "repo.toml")
	}
	tmplPath := flagTemplate
	if tmplPath == "" {
		tmplPath = filepath.Join(root, "templates", "index.html.tmpl")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	buildTime, err := build.BuildTimeFromEnv()
	if err != nil {
		return err
	}

	signed := os.Getenv("GPG_PRIVATE_KEY") != ""
	hasPublicKey := false
	if _, err := os.Stat(filepath.Join(root, "repo-public.key")); err == nil {
		hasPublicKey = true
	}
	return page.RenderLandingPage(cmd.Context(), output, cfg, tmplPath, buildTime, signed, hasPublicKey)
}
