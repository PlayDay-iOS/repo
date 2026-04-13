# PlayDay-iOS APT Repository

Debian-style repository for iOS package managers (Cydia, Zebra, Sileo), published through GitHub Pages.

Built as a single Go binary (`repotool`) with no external tool dependencies.

## Repository layout

- `pool/<suite>/<component>/`: source `.deb` files per suite. `.gitkeep` placeholders live under `pool/` only to keep empty directories tracked in git; the build creates pool subdirectories on demand and skips hidden files when mirroring into the published output.
- `repo.toml`: repository configuration (TOML)
- `templates/index.html.tmpl`: landing page template
- `resources/CydiaIcon.png`: source icon file (Made by [Evehly](https://www.deviantart.com/evehly/art/The-Last-Pringle-852158299))

Notes:

- `repo.name` and `repo.url` are required in `repo.toml`. `metadata.component` is a single string ("main" by default).
- Published suite roots use `./` source style (`deb <url>/<suite>/ ./`).
- Set `SOURCE_DATE_EPOCH` for reproducible builds (Unix timestamp). The build workflow derives this from the latest commit timestamp automatically. A non-empty but unparseable value is rejected.
- Index files are hashed with MD5, SHA1, SHA256, and SHA512 for compatibility with older clients.

## Requirements

- Go 1.26 or newer. The `toolchain` directive in `go.mod` lets older `go` binaries fetch the right toolchain when `GOTOOLCHAIN=auto` (the default).

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
  - `<suite>/pool/<suite>/<component>/*.deb` (mirror, if packages exist)
- `repo-public.key` (root, only if a `repo-public.key` file exists at the repo root; the landing page links to it only when this file is present)

Source lines:

- Stable: `deb https://playday-ios.github.io/repo/stable/ ./`
- Beta: `deb https://playday-ios.github.io/repo/beta/ ./`

## CLI

```sh
repotool build  [--output _site] [--config repo.toml] [--template templates/index.html.tmpl]
repotool import [--config repo.toml] [--allowlist org-import-allowlist.txt] [--suite <name>] [--include-prereleases] [--timeout 30m]
repotool render [--output _site] [--config repo.toml] [--template templates/index.html.tmpl]
repotool --version
```

Flag defaults are computed from the current working directory:

- `--output` defaults to `<cwd>/_site`
- `--config` defaults to `<cwd>/repo.toml`
- `--template` defaults to `<cwd>/templates/index.html.tmpl`
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
| `IMPORT_TIMEOUT`            | Go duration (e.g. `30m`, `2h`); default for `--timeout` on `import`.       |

### Configuration schema

```toml
[repo]
name = "PlayDay iOS Repo"          # required
url  = "https://example.com/repo/" # required, http(s) only

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
4. Imported packages are validated and committed under `pool/<target_suite>/<component>/`.
5. The build/deploy workflow publishes updated metadata.

Validation checks:

- Required control fields: `Package`, `Version`, `Architecture`, `Maintainer`, `Description`
- Allowed architectures from `repo.toml`
- Duplicate canonical names with different content are rejected
