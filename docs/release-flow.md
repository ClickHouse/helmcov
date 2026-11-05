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
4. `release-please.yml` calls `release.yml` with the new tag:
   - runs GoReleaser
   - publishes binaries/archives/checksums
   - publishes Docker image to GHCR
5. `release.yml` then runs a vendoring job:
   - downloads GoReleaser `dist/` artifact from the previous job
   - writes GHCR image refs
   - uploads everything as reusable workflow artifact
6. `release.yml` updates the floating `v1` Git tag used by `uses: .../helmcov@v1`.
7. `vendor-artifacts.yml` remains available as a manual fallback workflow.

Release Please calls `release.yml` directly because tags it creates with
`GITHUB_TOKEN` don't trigger tag-push workflows, so no PAT is needed.

## Files Involved

- `.github/workflows/release-please.yml`
- `.github/workflows/release.yml`
- `.github/workflows/vendor-artifacts.yml`
- `.goreleaser.yml`
- `release-please-config.json`
- `.release-please-manifest.json`
- `CHANGELOG.md`

## Repository Secrets

- `RELEASE_PLEASE_TOKEN` (optional): used instead of `GITHUB_TOKEN` when set.
  Not needed for publishing.

## Commit Convention and Version Bumps

Release Please infers semver bump type from Conventional Commits:

- `fix:` -> patch
- `feat:` -> minor
- `!` or `BREAKING CHANGE:` -> major

Examples:

- `fix(cli): handle empty tests path`
- `feat(report): add verbose uncovered line refs`
- `feat!: change default report format`

CI validates the Conventional Commit format of every PR title, including after
the title is edited, because the PR title becomes the squash commit on `main`.

## Tagging Convention

This repository uses a single semver convention everywhere:

- Git tag: `vX.Y.Z` (for example `v0.2.0`)
- GitHub release: `vX.Y.Z`
- GHCR image tag: `vX.Y.Z`

Release Please is configured with:

- `include-v-in-tag: true`
- `include-component-in-tag: false`

## First Release Bootstrap

If no releases exist yet:

1. Ensure at least one releasable commit exists on `main` (`feat`/`fix`).
2. Run or wait for `release-please.yml`.
3. Merge the generated release PR.

## Rebuilding a Release

To publish binaries for an existing release tag after a failed run, use
"Re-run failed jobs" on the release-please run, or dispatch `release.yml` with
the tag. Re-running the whole release-please run skips publishing, because the
release already exists.

```bash
gh workflow run release.yml -f tag=vX.Y.Z
```

The build uses the tag's own `.goreleaser.yml`, so this only works for v0.5.1
and later. Rebuilding a tag older than the latest release also moves `v1` and
the `latest` image back to it.

## Manual Vendoring

`vendor-artifacts.yml` supports manual dispatch for backfill/retry:

- Input `version` expects bare semver (for example `0.1.0`, without `v`).

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
