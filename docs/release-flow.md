# Release Flow

This project uses Release Please + GoReleaser for semver releases, Docker image
publishing to GHCR, and vendored release artifacts.

## Pipeline Overview

1. `main` receives new commits.
2. `release-please.yml` opens/updates a release PR:
   - bumps version
   - updates `CHANGELOG.md`
3. Merge release PR:
   - creates GitHub release + `vX.Y.Z` tag
4. `release.yml` triggers on `v*` tags:
   - runs GoReleaser
   - publishes binaries/archives/checksums
   - publishes Docker image to GHCR
5. `vendor-artifacts.yml` triggers on `release.published`:
   - downloads release assets
   - writes GHCR image refs
   - uploads everything as reusable workflow artifact

## Files Involved

- `.github/workflows/release-please.yml`
- `.github/workflows/release.yml`
- `.github/workflows/vendor-artifacts.yml`
- `.goreleaser.yml`
- `release-please-config.json`
- `.release-please-manifest.json`
- `CHANGELOG.md`

## Commit Convention and Version Bumps

Release Please infers semver bump type from Conventional Commits:

- `fix:` -> patch
- `feat:` -> minor
- `!` or `BREAKING CHANGE:` -> major

Examples:

- `fix(cli): handle empty tests path`
- `feat(report): add verbose uncovered line refs`
- `feat!: change default report format`

## First Release Bootstrap

If no releases exist yet:

1. Ensure at least one releasable commit exists on `main` (`feat`/`fix`).
2. Run or wait for `release-please.yml`.
3. Merge the generated release PR.

If you need to force release quickly, create a matching semver tag manually:

```bash
git tag -a v0.1.0 -m "v0.1.0"
git push origin v0.1.0
```

## Manual Vendoring

`vendor-artifacts.yml` also supports manual dispatch:

- Input `version` expects bare semver (for example `0.1.0`, without `v`).

The workflow resolves it to `v<version>` when downloading release assets.

## Published Artifacts

From GoReleaser:

- Multi-OS binaries and archives (linux/darwin/windows, amd64/arm64)
- `checksums.txt`
- Docker image:
  - `ghcr.io/<owner>/helmcov:vX.Y.Z`
  - `ghcr.io/<owner>/helmcov:latest`

From vendor-artifacts workflow:

- Uploaded workflow artifact named `helmcov-vX.Y.Z`
- Contains downloaded release assets and `image-refs.txt`
