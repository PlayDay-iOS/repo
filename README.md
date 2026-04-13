# PlayDay-iOS APT Repository

Debian-style repository for iOS package managers (Cydia, Zebra, Sileo), published through GitHub Pages.

Built as a single Go binary (`repotool`) with no external tool dependencies.

## Repository layout

- `pool/<suite>/<component>/`: source `.deb` files per suite. Tracked placeholders for `stable` and `beta` live under `pool/` to preserve the directories; rename or add more to match `metadata.suites` in `repo.toml`.
- `repo.toml`: repository configuration (TOML)
- `templates/index.html.tmpl`: landing page template
- `resources/CydiaIcon.png`: source icon file (Made by [Evehly](https://www.deviantart.com/evehly/art/The-Last-Pringle-852158299))

Notes:

- `repo.name` and `repo.url` are required in `repo.toml`.
- `metadata.components` currently must contain exactly one entry.
- Published suite roots use `./` source style (`deb <url>/<suite>/ ./`).
- Set `SOURCE_DATE_EPOCH` for reproducible builds (Unix timestamp). The build workflow derives this from the latest commit timestamp automatically.

## Build and publish

1. Add packages to `pool/<suite>/<component>/`, or use org import.
2. Build: `go build -o repotool ./cmd/repotool && ./repotool build --output _site --template templates/index.html.tmpl`
3. GitHub Actions deploys `_site/` to GitHub Pages.

Main workflow: `.github/workflows/build-and-deploy.yml`

## Quick start

1. Set `url` in `repo.toml` to the final Pages URL.
2. Add one `.deb` to `pool/stable/main/`.
3. Push to `main`.
4. In repository settings, enable Pages with source set to GitHub Actions.

Expected files after build (rooted at the output directory):

- `.repotool-output` (marker file at the output root)
- `CydiaIcon.png` (root)
- `index.html` (root landing page)
- Per suite (e.g. `stable/`, `beta/`):
  - `Packages` (+ `.gz`, `.xz`, `.bz2`)
  - `Release`, `Release.gpg`, `InRelease` (signed variants only when a key is supplied)
  - `CydiaIcon.png`
  - `index.html`
  - `pool/<suite>/<component>/*.deb` (mirror, if packages exist)

Source lines:

- Stable: `deb https://playday-ios.github.io/repo/stable/ ./`
- Beta: `deb https://playday-ios.github.io/repo/beta/ ./`

## CLI

```sh
repotool build   [--output _site] [--config repo.toml] [--template templates/index.html.tmpl]
repotool import  [--config repo.toml] [--allowlist org-import-allowlist.txt] [--suite <name>] [--include-prereleases]
repotool render  [--output _site] [--config repo.toml] [--template templates/index.html.tmpl]
repotool --version
```

The `--suite` flag on `import` defaults to the first entry of `metadata.suites` in `repo.toml`, or the `TARGET_SUITE` env var when set.

### Environment variables

| Variable                    | Purpose                                                                   |
| --------------------------- | ------------------------------------------------------------------------- |
| `SOURCE_DATE_EPOCH`         | Pins `Date:` and landing-page timestamp for reproducible builds.          |
| `GPG_PRIVATE_KEY`           | Armored signing key. Empty = signing skipped (no error).                  |
| `GPG_PASSPHRASE`            | Passphrase for `GPG_PRIVATE_KEY` when required.                           |
| `GPG_KEY_FILE`              | Overrides `signing.gpg_key_file` from `repo.toml`.                        |
| `GH_TOKEN` / `GITHUB_TOKEN` | GitHub API token for `import`; `GH_TOKEN` takes precedence.               |
| `GITHUB_API_BASE`           | Alternate GitHub API endpoint (e.g. GitHub Enterprise).                   |
| `ORG_NAME`                  | Overrides `github.org_name` from `repo.toml`.                             |
| `TARGET_SUITE`              | Default target suite for `import` when `--suite` is not passed.           |
| `INCLUDE_PRERELEASES`       | `true`/`false`; default for `--include-prereleases` when flag not passed. |

## Signing (optional)

`repotool` reads signing data from runtime env vars (`GPG_PRIVATE_KEY`, `GPG_PASSPHRASE`) or from `signing.gpg_key_file` in `repo.toml`. When no key is available, signing is silently skipped (only plain `Release` is written, no `Release.gpg` / `InRelease`).

To export the public key for client trust setup:

```sh
gpg --armor --export <key-id> > repo-public.key
```

The build copies `repo-public.key` (if present at the repo root) into the output directory.

In GitHub Actions, set these repository secrets (workflow maps them to runtime env vars above):

- `APT_GPG_PRIVATE_KEY`
- `APT_GPG_PASSPHRASE` (if key is protected)

## Org import

Files used by import:

- Allowlist: `org-import-allowlist.txt`
- Import workflow: `.github/workflows/import-org-packages.yml`

Required configuration:

- `github.org_name` in `repo.toml` (or `ORG_NAME` env var).
- A GitHub token via `GH_TOKEN` (recommended) or `GITHUB_TOKEN`. The import command errors out if either is missing because unauthenticated access is rate-limited to 60 requests per hour.

How it works:

1. Add allowed repository names to `org-import-allowlist.txt`.
2. Run the import workflow manually or wait for schedule.
3. Set `target_suite` to the desired suite name when running manually (defaults to `stable`).
4. Imported packages are validated and committed under `pool/<target_suite>/<component>/`.
5. The build/deploy workflow publishes updated metadata.

Validation checks:

- Required control fields: `Package`, `Version`, `Architecture`, `Maintainer`, `Description`
- Allowed architectures from `repo.toml`
- Duplicate canonical names with different content are rejected
