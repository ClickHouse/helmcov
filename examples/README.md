# Examples

This directory contains runnable sample Helm charts and helm-unittest suites for
helmcov development and onboarding.

- `basic-chart`: `include`, `tpl`, `fromYaml`, `.Files.Get`, and `.Capabilities.APIVersions`.
- `branch-heavy-chart`: nested `if`, `with`, and `range` branches plus `include`, `tpl`, and capabilities gates.
- `low-coverage-chart`: intentionally misses one `if` branch edge while exercising helpers, `tpl`, `fromYaml`, and chart files.
- `monorepo/charts/*`: nested chart layout for `--charts` mode with shared Helm function patterns.

Run from repo root:

```bash
go run ./cmd/helmcov --chart examples/basic-chart --tests examples/basic-chart/tests
go run ./cmd/helmcov --chart examples/branch-heavy-chart --tests examples/branch-heavy-chart/tests
go run ./cmd/helmcov --chart examples/low-coverage-chart --tests examples/low-coverage-chart/tests
go run ./cmd/helmcov --charts examples/monorepo/charts
```

Expected for `low-coverage-chart`: line coverage remains high, branch coverage
shows a gap (e.g. `branch-coverage=50.00%`) because the `prod` branch is not
exercised.

Use `--kube-version` when charts gate on Kubernetes version (for example
`chart-b` uses `semverCompare` against `.Capabilities.KubeVersion.GitVersion`).
