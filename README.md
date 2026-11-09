# helmcov

[![Release](https://img.shields.io/github/v/release/ClickHouse/helmcov)](https://github.com/ClickHouse/helmcov/releases)
[![Coverage](https://img.shields.io/endpoint?url=https://raw.githubusercontent.com/ClickHouse/helmcov/badges/coverage.json)](CONTRIBUTING.md)

helmcov measures line and branch coverage of Helm chart templates from your
[helm-unittest](https://github.com/helm-unittest/helm-unittest) suites. Helm
bugs tend to hide in conditionals and loops that only run under particular
values, and helmcov shows which of those your tests never reach.

It writes a Go coverprofile, Cobertura XML for CI dashboards, and a markdown
summary for pull request comments. A GitHub Action wraps it in one step.

## How it works

helmcov renders each template that the suites list, recording which lines
and branches run. It renders with:

- the chart's values plus each suite's `values` and `set`
- each test's own `values` and `set` on top
- up to `--max-scenarios` generated variants per suite that flip booleans and
  empty or fill lists and maps, to reach branches the suites miss

Rendering matches helm-unittest. The default release is `RELEASE-NAME` in
`NAMESPACE`, with Kubernetes 1.20 and no API versions, and any `release`,
`capabilities` and `chart` a suite or test sets apply on top. `templates` globs
such as `"*.yaml"` expand the same way. A chart that branches on
`.Capabilities.APIVersions.Has` needs its suites to list those versions under
`capabilities.apiVersions`, as helm-unittest itself requires. `lookup` answers
from the objects in a suite's or test's `kubernetesProvider`, and returns
nothing without them, as it does under `helm template`. Assertion options such
as `documentSelector` only choose which document an assertion checks, so they
do not change coverage.

Helm's built-ins work as they do in Helm, including `.Files`, `.helmignore`,
`tpl`, `toYaml` and how values files are parsed. An e2e test diffs helmcov's output
against `helm template` to keep it that way.

Branches are tracked for:

- `if` and `else if`, `with` and `else with`, and `range` (empty or not)
- each operand of `and` and `or` that gets evaluated
- whether `default` used its fallback, which side `ternary` took, and which
  `coalesce` argument was used

Each construct has its own keys, including two of the same kind on one line:
the second and later are numbered, as in `if#2:true`.

Lines that hold only `define`, `else` or `end`, comments, and blank lines are
not counted. Strings rendered through `tpl` are reported per file but left out
of the totals.

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

## Usage

```bash
helmcov --chart charts/my-app                  # suites in charts/my-app/tests
helmcov --chart charts/my-app --tests path/to/suites
helmcov --charts charts                        # every chart under charts/
helmcov --chart charts/my-app --threshold 80 --branch-threshold 70 --verbose
```

| Flag | Default | Meaning |
|------|---------|---------|
| `--chart` | | Chart directory (with `Chart.yaml`) |
| `--tests` | `<chart>/tests` | helm-unittest suites (`*_test.yaml`) |
| `--charts` | | Directory of charts, each with suites in `<chart>/tests`; replaces `--chart` |
| `--threshold` | `0` | Fail when line coverage is below this percentage |
| `--branch-threshold` | `0` | Fail when branch coverage is below this percentage. Both gates are checked and every miss is reported |
| `--format` | `go` and `cobertura` | Output format, repeatable: `go`, `cobertura`, `markdown` |
| `--go-coverprofile` | `coverage.out` | Go coverprofile path |
| `--cobertura-file` | `coverage.xml` | Cobertura XML path |
| `--markdown-file` | `coverage.md` | Markdown report path |
| `--markdown-file-summary` | off | Add a per-file table to the markdown report |
| `--markdown-include-tpl` | off | List `tpl` sources in the markdown report |
| `--comment-marker` | `helmcov-comment` | HTML comment that identifies the markdown report |
| `--max-scenarios` | `20` | Value scenarios per suite, including the suite's own values. `1` adds no generated variants, so coverage is close to what the tests alone reach |
| `--seed` | `42` | Seed for choosing generated variants |
| `--kube-version` | `1.20.0` | Kubernetes version for suites that set none |
| `--verbose` | off | Print each file's uncovered lines and branches with their source |
| `--version` | | Print the helmcov version and exit |

`--verbose` output names each gap as `path:line "source" (outcome)`, for
example `templates/service.yaml:12 "{{- if .Values.tls }}" (if:true)`.

## GitHub Action

The action installs a release binary, checks it against the release's
`checksums.txt`, runs helmcov, and keeps one pull request comment up to date
with the results.

```yaml
name: helm-coverage
on: pull_request

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
          threshold: "80"
          branch-threshold: "70"
```

For a repository with many charts, use `charts: charts` instead of `chart`.

| Input | Default | Meaning |
|-------|---------|---------|
| `chart` | | Chart directory |
| `tests` | `<chart>/tests` | helm-unittest suites |
| `charts` | | Directory of charts; replaces `chart` |
| `threshold` | `0` | Fail the step when line coverage is below this percentage |
| `branch-threshold` | `0` | Fail the step when branch coverage is below this percentage |
| `version` | `latest` | helmcov release tag, `latest`, or `dev` to build from the action's source |
| `comment` | `true` | Post or update the pull request comment |
| `comment-marker` | `helmcov-comment` | Identifies the comment to update |
| `token` | `github.token` | Token used to comment |
| `file-summary` | `false` | Add a per-file table to the comment |
| `include-tpl-sources` | `false` | List `tpl` sources in the comment |
| `markdown-file` | `helmcov-coverage.md` | Where to write the markdown report |
| `max-scenarios` | `20` | As `--max-scenarios` |
| `seed` | `42` | As `--seed` |
| `kube-version` | `1.20.0` | As `--kube-version` |
| `repository` | `ClickHouse/helmcov` | Repository to download releases from |

Outputs: `line-coverage`, `branch-coverage` (percentages) and `markdown-file`.

To run the CLI in CI without the action, download a release or use
`go install`, then upload `coverage.xml` to your coverage service.

## Coverage reports

- Cobertura XML is read by GitLab's coverage visualisation, Jenkins and
  Codecov. SonarQube's generic coverage import uses a different format, so it
  needs a conversion step.
- The Go coverprofile uses Go's `mode: count` format with template paths in
  place of Go files, for tools that merge or diff profiles. `go tool cover`
  itself needs Go sources and cannot render it.
- The markdown report is what the action posts on pull requests.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md); coding agents should read
[AGENTS.md](AGENTS.md). Releases are described in
[docs/release-flow.md](docs/release-flow.md).

## Licence

Apache-2.0. See [LICENSE](LICENSE).
