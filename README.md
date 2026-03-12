# helmcov

Dynamic Helm template coverage for branch-heavy charts.

`helmcov` executes Helm templates with chart values, helm-unittest suite values,
and generated branch-oriented scenarios to estimate:

- line coverage in template files
- branch coverage for `if`, `range`, and `with`

It emits:

- native Go `coverprofile`
- Cobertura XML (for CI/reporting systems)

## Why this exists

Helm failures often live in conditionals and loops that only show up under
specific values. `helmcov` focuses on those divergent branches.

## Install

### Binary

Build from source:

```bash
go build -o helmcov ./cmd/helmcov
```

### Docker

```bash
docker build -t helmcov:dev .
```

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

- `--format`: output format (`go`, `cobertura`), repeatable
  - default: both `go` and `cobertura`
- `--go-coverprofile`: output file for Go coverprofile
  - default: `coverage.out`
- `--cobertura-file`: output file for Cobertura XML
  - default: `coverage.xml`
- `--threshold`: minimum line coverage percentage (0-100)
  - command fails if actual line coverage is lower
- `--max-scenarios`: cap for generated value scenarios per suite
  - default: `20`
- `--seed`: random seed for deterministic scenario ordering
  - default: `42`
- `--verbose`: print CI-friendly per-file details, including uncovered lines and branch edges
  - includes exact uncovered line refs as `path:line("source")`

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

### Binary mode

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

### Docker mode

```yaml
name: helmcov-docker
on: [pull_request]
jobs:
  coverage:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: docker build -t helmcov:ci .
      - run: |
          docker run --rm -v "$PWD:/work" -w /work helmcov:ci \
            --chart examples/basic-chart \
            --tests examples/basic-chart/tests
```

## Release and vendoring workflows

- `.github/workflows/release-please.yml`: maintains semver releases and
  `CHANGELOG.md` using Release Please.
- `.github/workflows/release.yml`: publishes multi-OS binaries and Docker image
  through GoReleaser on tag push.
- `.github/workflows/vendor-artifacts.yml`: downloads a tagged release and
  uploads binaries plus image references as reusable artifacts. Runs on release
  publish automatically and can also be triggered manually.

## Semver workflow

1. Merge changes into `main`.
2. Release Please opens/updates a release PR with version bump + changelog.
3. Merge that PR to create `vX.Y.Z` tag and GitHub release.
4. `release.yml` publishes binaries and GHCR image tags for that version.
5. `vendor-artifacts.yml` runs on release publish and uploads vendored assets.

Release Please infers bump level from Conventional Commit prefixes (`feat`,
`fix`, and `!`/`BREAKING CHANGE`).

## SonarQube integration notes

SonarQube does not natively parse Helm template coverage. The pragmatic path:

1. generate Cobertura XML from `helmcov`
2. feed Cobertura XML into your reporting pipeline as generic coverage input
3. map/report results at CI stage until native Helm support exists

## Project principles

- TDD-first implementation
- deterministic output and scenario generation
- package boundaries designed for extension (CLI, loader, tracing, reporters)
