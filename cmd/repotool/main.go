package main

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
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
	flagBuildOutput   string
	flagBuildTemplate string
	flagAllowlist     string
	flagSuite         string
	flagPrereleases   bool
	flagRenderOutput  string
	flagRenderTmpl    string
)

func init() {
	rootCmd.PersistentFlags().StringVar(&configPath, "config", "", "Path to repo.toml")

	buildCmd.Flags().StringVar(&flagBuildOutput, "output", "", "Output directory (default: <root>/_site)")
	buildCmd.Flags().StringVar(&flagBuildTemplate, "template", "", "Path to HTML template")

	importCmd.Flags().StringVar(&flagAllowlist, "allowlist", "", "Path to allowlist file")
	importCmd.Flags().StringVar(&flagSuite, "suite", "", "Target suite (default from config or TARGET_SUITE env)")
	importCmd.Flags().BoolVar(&flagPrereleases, "include-prereleases", false, "Include prerelease assets")

	renderCmd.Flags().StringVar(&flagRenderOutput, "output", "", "Output directory")
	renderCmd.Flags().StringVar(&flagRenderTmpl, "template", "", "Path to HTML template")

	rootCmd.AddCommand(buildCmd, importCmd, renderCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func runBuild(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	output := flagBuildOutput
	if output == "" {
		output = filepath.Join(root, "_site")
	}
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(root, "repo.toml")
	}
	tmplPath := flagBuildTemplate
	if tmplPath == "" {
		tmplPath = filepath.Join(root, "templates", "index.html.tmpl")
	}

	return build.Run(build.Options{
		RootDir:      root,
		OutputDir:    output,
		ConfigPath:   cfgPath,
		TemplatePath: tmplPath,
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

	opts := ghimport.Options{
		RootDir:            root,
		ConfigPath:         cfgPath,
		AllowlistPath:      allowlist,
		Suite:              flagSuite,
		IncludePrereleases: flagPrereleases,
	}

	// Env overrides
	if v := os.Getenv("TARGET_SUITE"); v != "" && flagSuite == "" {
		opts.Suite = v
	}
	if envVal := os.Getenv("INCLUDE_PRERELEASES"); envVal != "" && !cmd.Flags().Changed("include-prereleases") {
		if v, parseErr := strconv.ParseBool(envVal); parseErr == nil {
			opts.IncludePrereleases = v
		}
	}

	return ghimport.Run(opts)
}

func runRender(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}

	output := flagRenderOutput
	if output == "" {
		output = filepath.Join(root, "_site")
	}
	cfgPath := configPath
	if cfgPath == "" {
		cfgPath = filepath.Join(root, "repo.toml")
	}
	tmplPath := flagRenderTmpl
	if tmplPath == "" {
		tmplPath = filepath.Join(root, "templates", "index.html.tmpl")
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}

	return page.RenderLandingPage(output, cfg, tmplPath, build.ResolveBuildTime(time.Time{}))
}
