# helmcov

[![Release](https://img.shields.io/github/v/release/ClickHouse/helmcov)](https://github.com/ClickHouse/helmcov/releases)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/ClickHouse/helmcov/badges/coverage.json)](#development-workflow)

Dynamic Helm template coverage for branch-heavy charts.

`helmcov` executes Helm templates with chart values, helm-unittest suite values,
each test's own `values` and `set`, and generated branch-oriented scenarios to
estimate:

- line coverage in template files
- branch coverage for `if`/`else if`, `range`, `with`/`else with`, each
  `and`/`or` operand, and the outcomes of `default`, `ternary` and `coalesce`

Templates render the way helm-unittest renders them: with its default release
(`RELEASE-NAME` in `NAMESPACE`), Kubernetes 1.20 and no API versions, plus any
`release`, `capabilities` and `chart` a suite or test sets, and with
`templates` globs such as `"*.yaml"` expanded. A chart that branches on
`.Capabilities.APIVersions.Has` needs its suites to list those versions under
`capabilities.apiVersions`, as helm-unittest itself does. `--kube-version`
sets the Kubernetes version for suites that do not.

Coverage counts the chart's own templates. Suites may also list a packaged
dependency's templates (`charts/<subchart>/templates/...`); helmcov skips those.

It emits:

- native Go `coverprofile`
- Cobertura XML (for CI/reporting systems)

## Why this exists

Helm failures often live in conditionals and loops that only show up under
specific values. `helmcov` focuses on those divergent branches.

## Install

Download a binary for Linux, macOS or Windows from the
[releases page](https://github.com/ClickHouse/helmcov/releases), or:

```bash
go install github.com/ClickHouse/helmcov/cmd/helmcov@latest
```

Release archives carry build provenance, so you can check one was built by
this repository's release workflow:

```bash
gh attestation verify helmcov_<version>_<os>_<arch>.tar.gz --repo ClickHouse/helmcov
```

The GitHub Action checks each download against the release's `checksums.txt`
before running it.

## Development workflow

Use the repo `Makefile` to run local checks and setup:

```bash
make setup
make ci
```

`make setup` installs a native git `commit-msg` hook and runs `go mod tidy`.
`make ci` runs formatting checks, linting, tests (including integration tests),
and build.

## Usage

```bash
helmcov --chart <chart-dir> --tests <tests-dir> [flags]
helmcov --charts <charts-dir> [flags]
```

### Required flags

- `--chart`: path to chart root directory (must include `Chart.yaml`)
- `--tests`: path to helm-unittest suites (must include at least one `*_test.yaml`)
  - optional in `--chart` mode; defaults to `<chart>/tests`
- `--charts`: root directory containing one or more nested charts (recursive `Chart.yaml` discovery)
- `--charts-root`: deprecated alias for `--charts`

Mode rules:

- Single-chart mode: use `--chart` with `--tests`
- Monorepo mode: use `--charts` (tests are auto-discovered as `<chart>/tests`)
- `--chart` and `--charts` are mutually exclusive

### Optional flags

- `--format`: output format (`go`, `cobertura`, `markdown`), repeatable
  - default: both `go` and `cobertura`
- `--go-coverprofile`: output file for Go coverprofile
  - default: `coverage.out`
- `--cobertura-file`: output file for Cobertura XML
  - default: `coverage.xml`
- `--markdown-file`: output file for markdown PR comment report
  - default: `coverage.md`
- `--comment-marker`: HTML comment marker embedded in markdown output
  - default: `helmcov-comment`
- `--threshold`: minimum line coverage percentage (0-100)
  - command fails if actual line coverage is lower
- `--max-scenarios`: cap for value scenarios per suite, including the suite's
  own values. Each helm-unittest test also renders with its own values on top.
  `1` adds no generated variants, so coverage is close to what the tests
  exercise.
  - default: `20`
- `--seed`: random seed for deterministic scenario ordering
  - default: `42`
- `--verbose`: print CI-friendly per-file details, including uncovered lines and branch edges
  - includes exact uncovered line refs as `path:line("source")`
- `--version`: print the helmcov version and exit

## Example commands

Run against included examples:

```bash
go run ./cmd/helmcov --chart examples/basic-chart --tests examples/basic-chart/tests
go run ./cmd/helmcov --chart examples/branch-heavy-chart --tests examples/branch-heavy-chart/tests
go run ./cmd/helmcov --charts examples/monorepo/charts
```

Write both outputs to custom locations:

```bash
go run ./cmd/helmcov \
  --chart examples/branch-heavy-chart \
  --tests examples/branch-heavy-chart/tests \
  --format go \
  --format cobertura \
  --go-coverprofile out/helm.coverage.out \
  --cobertura-file out/helm.cobertura.xml \
  --threshold 70
```

## GitHub Actions integration

### Published action (recommended)

Add Helm template coverage to pull requests with a single step. The action
installs a release binary, runs helmcov, and upserts a PR comment with
line/branch coverage and uncovered details.

```yaml
name: helm-coverage
on:
  pull_request:

permissions:
  contents: read
  pull-requests: write

jobs:
  helmcov:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: ClickHouse/helmcov@v1
        with:
          chart: charts/my-app
          tests: charts/my-app/tests
          threshold: "70"
```

Monorepo mode:

```yaml
      - uses: ClickHouse/helmcov@v1
        with:
          charts: charts
          threshold: "65"
```

Action inputs:

| Input | Description | Default |
|-------|-------------|---------|
| `chart` | Path to chart root | — |
| `tests` | Path to helm-unittest suites | `<chart>/tests` |
| `charts` | Monorepo charts root | — |
| `threshold` | Minimum line coverage % | `0` |
| `version` | Release tag (`v0.4.2`), `latest`, or `dev` | `latest` |
| `comment` | Post/update PR comment | `true` |
| `comment-marker` | Marker for comment upsert | `helmcov-comment` |

Requirements for consuming repositories:

- helm-unittest suites (`*_test.yaml`) under the chart tests directory
- `pull-requests: write` when PR comments are enabled

This repository dogfoods the action in `.github/workflows/helmcov-action.yml`
using `version: dev` to build from source on pull requests.

### CI in this repository

CI (`.github/workflows/ci.yml`) runs on pull requests and pushes to `main`, and enforces:

- `go mod tidy` and `gofmt` checks
- `go vet` and a pinned `golangci-lint` (config in `.golangci.yml`)
- the full Go test suite with the race detector, an 85% coverage minimum, and
  the integration tests
- build validation
- `govulncheck` against known Go vulnerabilities
- Conventional Commit PR titles on every pull request (`pr-title.yml`)

The workflow executes `make ci` so CI and local checks stay aligned.

### Manual workflow (binary)

```yaml
name: helmcov
on: [pull_request]
jobs:
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - run: go build -o helmcov ./cmd/helmcov
      - run: |
          ./helmcov \
            --chart examples/branch-heavy-chart \
            --tests examples/branch-heavy-chart/tests \
            --format go \
            --format cobertura
      - uses: actions/upload-artifact@v4
        with:
          name: helmcov-reports
          path: |
            coverage.out
            coverage.xml
```

## Local commit-msg gate

Conventional commit validation runs locally via a native git `commit-msg` hook.

Setup:

```bash
make hooks
```

The hook source lives in `.githooks/commit-msg` and validates commit subjects
with `scripts/validate-conventional-commit.sh`.

## Release and vendoring workflows

- `.github/workflows/release-please.yml`: maintains semver releases and
  `CHANGELOG.md` using Release Please.
- `.github/workflows/release.yml`: publishes multi-OS binaries and Docker image
  through GoReleaser when Release Please cuts a release, then uploads vendored
  artifacts as a second job using the GoReleaser `dist/` artifact handoff. Run
  it manually with a `tag` input to rebuild an existing release.
- `.github/workflows/vendor-artifacts.yml`: downloads a tagged release and
  uploads binaries plus image references as reusable artifacts. Manual fallback
  workflow for backfills and retries.
- Detailed runbook: `docs/release-flow.md`.

## Semver workflow

1. Merge changes into `main`.
2. Release Please opens/updates a release PR with version bump + changelog.
3. Merge that PR to create `vX.Y.Z` tag and GitHub release.
4. `release.yml` publishes binaries and GHCR image tags for that version.
5. `release.yml` vendor job downloads release assets and uploads vendored
   artifacts.
6. Optional: run `vendor-artifacts.yml` manually for backfill/retry.

Release Please infers bump level from Conventional Commit prefixes (`feat`,
`fix`, and `!`/`BREAKING CHANGE`).

Tag and image convention is `vX.Y.Z`:

- Git tag: `vX.Y.Z`
- GitHub release: `vX.Y.Z`
- GHCR image: `ghcr.io/<owner>/helmcov:vX.Y.Z` (+ `latest`)

## SonarQube integration notes

SonarQube does not natively parse Helm template coverage. The pragmatic path:

1. generate Cobertura XML from `helmcov`
2. feed Cobertura XML into your reporting pipeline as generic coverage input
3. map/report results at CI stage until native Helm support exists

## Contributing

See `CONTRIBUTING.md`. Coding agents should read `AGENTS.md`.

## Licence

Apache-2.0. See `LICENSE`.
