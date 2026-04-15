# PlayDay-iOS APT Repository

Debian-style repository for iOS package managers (Cydia, Zebra, Sileo), published through GitHub Pages.

Built as a single Go binary (`repotool`) with no external tool dependencies.

## Repository layout

- `pool/<suite>/<component>/`: source `.deb` files per suite. `.gitkeep` placeholders keep empty directories tracked in git; the build mirrors only validated `.deb` files into the published output, so placeholders never leak.
- `repo.toml`: repository configuration (TOML)
- `internal/page/templates/index.html.tmpl`: landing page template, embedded into `repotool` at build time. Override with `--template <path>`.
- `resources/CydiaIcon.png`: source icon file (Made by [Evehly](https://www.deviantart.com/evehly/art/The-Last-Pringle-852158299))

Notes:

- `repo.name` and `repo.url` are required in `repo.toml`. `repo.url` must use `https://`. `metadata.component` is a single string ("main" by default).
- Published suite roots use `./` source style (`deb <url>/<suite>/ ./`).
- Set `SOURCE_DATE_EPOCH` for reproducible builds (Unix timestamp). The build workflow derives this from the latest commit timestamp automatically. A non-empty but unparseable value is rejected.
- Index files are hashed with MD5, SHA1, SHA256, and SHA512 for compatibility with older clients.

## Requirements

- Go 1.26 or newer. The `go 1.26.0` directive in `go.mod` triggers automatic toolchain download when `GOTOOLCHAIN=auto` (the default since Go 1.21).

## Build and publish

1. Add packages to `pool/<suite>/<component>/`, or use org import.
2. Build: `go build -o repotool ./cmd/repotool && ./repotool build`
3. GitHub Actions deploys `_site/` to GitHub Pages.

Main workflow: `.github/workflows/build-and-deploy.yml`

## Quick start

1. Set `repo.url` in `repo.toml` to the final Pages URL.
2. Add one `.deb` to `pool/stable/main/`.
3. Push to `main`.
4. In repository settings, enable Pages with source set to GitHub Actions.

Expected files after build (rooted at the output directory):

- `.repotool-output` — marker file written at the output root. `repotool build` refuses to wipe an existing output directory unless this marker is present, so an accidental `--output ~/important-stuff` is rejected.
- `CydiaIcon.png` (root)
- `index.html` (root landing page)
- Per suite (e.g. `stable/`, `beta/`):
  - `Packages` (+ `.gz`, `.xz`, `.bz2`)
  - `Release`, `Release.gpg`, `InRelease` (signed variants only when a key is supplied)
  - `CydiaIcon.png`
  - `index.html`
  - `pool/<suite>/<component>/*.deb` (mirror of validated packages, keeping the filenames from the source pool; absent when the suite has none)
- `repo-public.key` (root, only if a `repo-public.key` file exists at the repo root; the landing page links to it only when this file is present)
- `depictions/` (when at least one suite has entries):
  - `style.css` — shared stylesheet for HTML depictions
  - `<Package>/<VersionEscaped>/depiction.html` — Cydia HTML depiction
  - `<Package>/<VersionEscaped>/sileo.json` — Sileo native depiction JSON

Source lines:

- Stable: `deb https://playday-ios.github.io/repo/stable/ ./`
- Beta: `deb https://playday-ios.github.io/repo/beta/ ./`

## Depictions

`repotool build` auto-generates Cydia HTML and Sileo native JSON depictions for every version in the pool. Each entry's `Packages` stanza gets `Depiction:` and `SileoDepiction:` URLs injected that point at the generated files.

- Content is derived from the `.deb` control file only — no per-package source files.
- A compatibility banner ("iOS X.Y – Z.W") is rendered when the control sets `Depends: firmware (>= X), firmware (<< Y)` clauses, or when a free-text `X-Supported-iOS:` field is present.
- If the control already had a `Depiction:` value, it is preserved under `Homepage:` when `Homepage:` was empty.

## CLI

```sh
repotool build  [--output _site] [--config repo.toml] [--template <path>] [--depiction-template <path>] [--depiction-style <path>]
repotool import [--config repo.toml] [--allowlist org-import-allowlist.txt] [--suite <name>] [--include-prereleases] [--timeout 30m]
repotool render [--output _site] [--config repo.toml] [--template <path>]
repotool --version
```

Flag defaults:

- `--output` defaults to `<cwd>/_site`
- `--config` defaults to `<cwd>/repo.toml`
- `--template` is empty by default — `repotool` renders the embedded template. Pass a file path to override.
- `--depiction-template` is empty by default — `repotool` uses the embedded depiction HTML template. Pass a file path to override.
- `--depiction-style` is empty by default — `repotool` uses the embedded depiction stylesheet. Pass a file path to override.
- `--allowlist` defaults to `<cwd>/org-import-allowlist.txt`

The `--suite` flag on `import` defaults to the first entry of `metadata.suites` in `repo.toml`, or the `TARGET_SUITE` env var when set.

### Environment variables

| Variable                    | Purpose                                                                   |
| --------------------------- | ------------------------------------------------------------------------- |
| `SOURCE_DATE_EPOCH`         | Pins `Date:` and landing-page timestamp for reproducible builds.          |
| `GPG_PRIVATE_KEY`           | Armored signing key. Empty = signing skipped (no error).                  |
| `GPG_PASSPHRASE`            | Passphrase for `GPG_PRIVATE_KEY` when required.                           |
| `GH_TOKEN` / `GITHUB_TOKEN` | GitHub API token for `import`; `GH_TOKEN` takes precedence.               |
| `GITHUB_API_BASE`           | Alternate GitHub API endpoint (e.g. GitHub Enterprise).                   |
| `ORG_NAME`                  | Overrides `github.org_name` from `repo.toml`.                             |
| `TARGET_SUITE`              | Default target suite for `import` when `--suite` is not passed.           |
| `INCLUDE_PRERELEASES`       | `true`/`false`; default for `--include-prereleases` when flag not passed. |
| `IMPORT_TIMEOUT`            | Go duration (e.g. `30m`, `2h`); default for `--timeout` on `import`.      |

### Configuration schema

```toml
[repo]
name = "PlayDay iOS Repo"          # required
url  = "https://example.com/repo/" # required, https:// only

[metadata]
origin        = "PlayDay-iOS"             # optional, written into Release
label         = "PlayDay-iOS"             # optional, written into Release
description   = "..."                     # optional, written into Release
suites        = ["stable", "beta"]        # default: ["stable"]
component     = "main"                    # default: "main"; single string
architectures = ["iphoneos-arm64", "all"] # example override; default is ["iphoneos-arm", "iphoneos-arm64", "all"]

[github]
org_name = "PlayDay-iOS"  # required only for `import`
```

## Signing (optional)

`repotool` reads signing data from runtime env vars (`GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`). When no key is provided, signing is silently skipped: only the plain `Release` is written, and no `Release.gpg` / `InRelease` is produced.

To export the public key for client trust setup:

```sh
gpg --armor --export <key-id> > repo-public.key
```

The build copies `repo-public.key` (if present at the repo root) into the output directory. The landing page links to it only when the file is present, so unsigned repos do not show a dead link.

In GitHub Actions, set these repository secrets (workflow maps them to the runtime env vars above):

- `APT_GPG_PRIVATE_KEY`
- `APT_GPG_PASSPHRASE` (if key is protected)

To load a key from a file rather than env var, expand it inline in the shell that invokes `repotool`:

```sh
GPG_PRIVATE_KEY="$(cat key.asc)" ./repotool build
```

## Org import

Files used by import:

- Allowlist: `org-import-allowlist.txt`
- Import workflow: `.github/workflows/import-org-packages.yml`

Required configuration:

- `github.org_name` in `repo.toml` (or `ORG_NAME` env var).
- A GitHub token via `GH_TOKEN` (recommended) or `GITHUB_TOKEN`. The import command errors out if either is missing because unauthenticated access is rate-limited to 60 requests per hour.

How it works:

1. Add allowed repository names to `org-import-allowlist.txt`.
2. Run the import workflow manually or wait for the schedule.
3. Set `target_suite` to the desired suite name when running manually (defaults to `stable`).
4. Imported packages are validated and placed into `pool/<target_suite>/<component>/`.
5. The import workflow commits new packages and triggers the build/deploy workflow, which publishes updated metadata.

Validation checks:

- Required control fields: `Package`, `Version`, `Architecture`, `Maintainer`, `Description`
- Allowed architectures from `repo.toml`
- Duplicate canonical names with different content are rejected
