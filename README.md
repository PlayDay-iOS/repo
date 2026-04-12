# PlayDay-iOS APT Repository

Debian-style repository for iOS package managers (Cydia, Zebra, Sileo), published through GitHub Pages.

Built as a single Go binary (`repotool`) with no external tool dependencies.

## Repository layout

- `pool/stable/main/`: stable channel packages
- `pool/beta/main/`: beta channel packages
- `repo.toml`: repository configuration (TOML)
- `templates/index.html.tmpl`: landing page template
- `resources/CydiaIcon.png`: source icon file (Made by [Evehly](https://www.deviantart.com/evehly/art/The-Last-Pringle-852158299))

Notes:

- `metadata.components` currently must contain exactly one entry.
- Published suite roots use `./` source style (`deb <url>/<suite>/ ./`).

## Build and publish

1. Add packages to `pool/stable/main/` or `pool/beta/main/`, or use org import.
2. Build: `go build -o repotool ./cmd/repotool && ./repotool build --output _site --template templates/index.html.tmpl`
3. GitHub Actions deploys `_site/` to GitHub Pages.

Main workflow: `.github/workflows/build-and-deploy.yml`

## Quick start

1. Set `url` in `repo.toml` to the final Pages URL.
2. Add one `.deb` to `pool/stable/main/`.
3. Push to `main`.
4. In repository settings, enable Pages with source set to GitHub Actions.

Expected files after build:

- `.repotool-output`
- `stable/Packages` (+ `.gz`, `.xz`, `.bz2`)
- `stable/Release`
- `stable/CydiaIcon.png`
- `stable/index.html`
- `stable/pool/stable/main/*.deb` (mirror, if packages exist)
- `beta/Packages` (+ `.gz`, `.xz`, `.bz2`)
- `beta/Release`
- `beta/CydiaIcon.png`
- `beta/index.html`
- `beta/pool/beta/main/*.deb` (mirror, if packages exist)
- `CydiaIcon.png`
- `index.html`

Source lines:

- Stable: `deb https://playday-ios.github.io/repo/stable/ ./`
- Beta: `deb https://playday-ios.github.io/repo/beta/ ./`

## CLI

```sh
repotool build   [--output _site] [--config repo.toml] [--template templates/index.html.tmpl]
repotool import  [--config repo.toml] [--allowlist org-import-allowlist.txt] [--suite stable] [--include-prereleases]
repotool render  [--output _site] [--config repo.toml] [--template templates/index.html.tmpl]
repotool --version
```

## Signing (optional)

`repotool` reads signing data from runtime env vars:

- `GPG_PRIVATE_KEY`
- `GPG_PASSPHRASE` (if key is protected)

Alternative key source:

- `signing.gpg_key_file` in `repo.toml` (or env override `GPG_KEY_FILE`)

In GitHub Actions, set these repository secrets (workflow maps them to runtime env vars above):

- `APT_GPG_PRIVATE_KEY`
- `APT_GPG_PASSPHRASE` (if key is protected)

## Org import

Files used by import:

- Allowlist: `org-import-allowlist.txt`
- Import workflow: `.github/workflows/import-org-packages.yml`

How it works:

1. Add allowed repository names to `org-import-allowlist.txt`.
2. Run import workflow manually or wait for schedule.
3. Choose `target_suite` (`stable` or `beta`) when running manually.
4. Imported packages are validated and committed under `pool/<target_suite>/main/`.
5. Build/deploy workflow publishes updated metadata.

Validation checks:

- Required control fields: `Package`, `Version`, `Architecture`, `Maintainer`, `Description`
- Allowed architectures from `repo.toml`
- Duplicate canonical names with different content are rejected
