---
name: golang-dev
description: Build and maintain helmcov in Go with strict TDD, SOLID package boundaries, deterministic coverage behavior, and release-ready binary/docker workflows. Use when adding Go features, tests, CI, packaging, or coverage/reporting logic.
---

# Helmcov Go Development

## Core Workflow

1. Write a failing test first.
2. Run only that test and confirm failure reason is correct.
3. Implement minimal code to pass.
4. Run focused tests, then `go test ./...`.
5. Refactor only after green.

## Architecture Rules

- Keep parser/execution/reporting concerns in separate packages.
- Prefer narrow interfaces over concrete coupling.
- Keep CLI thin; orchestration lives in internal packages.
- Preserve deterministic behavior for suite discovery, scenario generation, and output ordering.

## Coverage Semantics

- Track line hits and branch edges (`if:true/false`, `range:empty/non-empty`, `with:empty/non-empty`).
- Keep trace keys stable across runs.
- Normalize coverage into a single model before writing formats.

## Testing Rules

- Use table-driven tests where input matrices are useful.
- Keep fixtures minimal and local to tests unless shared by multiple packages.
- Add golden output checks for report formats.

## CI and Release Rules

- CI baseline: `go test ./...` and build `./cmd/helmcov`.
- Release with GoReleaser for multi-OS binaries.
- Keep Docker image minimal with static binary entrypoint.
- Keep GitHub Actions workflows explicit and composable.
