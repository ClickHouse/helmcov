# Integration Guide

## Integrating into an existing chart repository

1. Ensure helm-unittest suites exist for your chart.
2. Add a CI step that runs:

```bash
helmcov --chart <chart-root> --tests <chart-root>/tests --format go --format cobertura
```

3. Upload `coverage.out` and `coverage.xml` as build artifacts.
4. Add `--threshold` once baseline coverage is understood.

## Recommended rollout

- Phase 1: run with no threshold and collect baseline.
- Phase 2: set threshold below baseline to stabilize.
- Phase 3: ratchet threshold upward as tests improve.

## Scope and non-goals

- Explicit chart/test paths only (`--chart`, `--tests`).
- No monorepo auto-discovery in current version.
- Coverage is focused on Helm templates and control-flow branches.
