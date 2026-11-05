# Working on helmcov

helmcov measures line and branch coverage of Helm chart templates. It renders
templates with chart values, helm-unittest suite values and generated scenarios,
then writes Go coverprofile, Cobertura XML and a markdown PR comment.

Read this file before changing code. `CONTRIBUTING.md` covers the same ground
for humans; this file is the stricter version.

## Layout

| Path | Owns |
|------|------|
| `cmd/helmcov` | `main` only: parse args, call `cli.Run`, set the exit code |
| `internal/cli` | Flag parsing, target discovery, orchestration of one run |
| `internal/chartloader` | Reading charts, values, helm-unittest suites, `.Files`, `.Capabilities` |
| `internal/instrumentation` | Rendering templates with the Helm function map and recording line and branch hits |
| `internal/coverage` | The coverage model every reporter reads |
| `internal/reporters` | Go coverprofile, Cobertura XML, markdown |
| `internal/valuegen` | Generating value scenarios that flip branches |
| `examples/` | Charts used by the integration tests and the GitHub Action self-test |
| `action.yml`, `scripts/action-*.sh` | The composite GitHub Action |

Everything is under `internal/`, so there is no public Go API to keep stable.
The CLI flags, the output formats and the action inputs are the public contract.

## Commands

| Command | Use |
|---------|-----|
| `make setup` | Install the commit-msg hook and tidy modules, once per clone |
| `make test` | All Go tests |
| `make integration-test` | The CLI run against `examples/` |
| `make mutation` | Mutation testing on every `internal/` package with gremlins |
| `make e2e` | The built binary against `e2e/testdata`, plus a render diff against `helm template` |
| `make cover` | Tests with coverage; fails below 85% |
| `make lint` | `go vet` plus golangci-lint |
| `make ci` | What CI runs. Run it before pushing |

## Workflow

1. Write a failing test that shows the bug or the missing behaviour.
2. Run only that test and check it fails for the reason you expect.
3. Make the smallest change that passes it.
4. Run `make ci`.
5. Refactor only while green, in a separate commit if it is large.

Keep a pull request to one concern. A bug fix does not also rename packages.

## Go conventions

- Format with `gofmt`. Do not hand-align code.
- Wrap errors with context: `fmt.Errorf("read template %s: %w", path, err)`.
  Messages are lowercase with no trailing punctuation. Never discard an error
  without a comment saying why.
- No `panic` outside `main` and test helpers. Return errors.
- No package-level mutable state. Pass dependencies in.
- Output must be deterministic. Sort map keys before iterating anything that
  reaches a report, a trace key or a test assertion.
- Accept interfaces, return concrete types. Define an interface in the package
  that consumes it, and only when there are two implementations or a test needs
  a fake.
- Exported identifiers get a doc comment that starts with the identifier's name.
- Comments say why, in one or two lines. Do not restate the code.
- Use the standard library first. A new module dependency needs a reason in the
  PR description; the shipped binary carries every dependency.
- `go.mod`'s `go` directive is the minimum supported Go version. Do not raise it
  without a reason, such as a language feature or a security fix.

## Workflows

- Pin every third-party action to a full commit SHA with its version in a
  comment (`uses: actions/checkout@<sha> # v7.0.1`). Dependabot updates both.
- Give each workflow `permissions: contents: read` and widen it per job only
  where the job needs it.
- Read untrusted event fields (PR titles, branch names) through `env:`, never
  by interpolating `${{ }}` into a `run:` script.

## Coverage semantics

Trace keys are part of the output contract. Changing their format changes every
user's reports.

- Line key: `<template path>:<line>`.
- Branch key: `<template path>:<line>:<construct>:<outcome>`, for example
  `if:true`/`if:false`, `range:empty`/`range:non-empty`,
  `with:empty`/`with:non-empty`.
- Hits are recorded while the template executes, so conditions see the real
  dot, variables and `include` arguments.
- A branch counts as covered when its hit count is above zero in any scenario.
  Every branch is registered, including ones nested under a branch that never
  ran.
- A line is coverable when it holds template text or starts an action. Blank
  lines, comments and lines holding only `define`, `else` or `end` are not.
- `define` blocks are reported against the file that defines them. The top
  level of a helper file never renders, so it is not reported.

When you add a construct, add its keys to this list in the same PR.

## Testing

- Use table-driven tests with `t.Run` and named cases. Call `t.Parallel()`
  unless the test changes process state.
- Build fixtures with `t.TempDir()`. Do not write into the repository from a
  test.
- Put chart fixtures shared by several tests under `examples/` or a package's
  `testdata/` directory. Keep inline fixtures for single-test cases.
- Assert exact trace keys and hit counts, not just percentages. A percentage
  can stay the same while the wrong branch is counted.
- Check that new tests catch bugs, not just run code, with
  [gremlins](https://github.com/go-gremlins/gremlins) mutation testing. Install
  it with `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`,
  then run `gremlins unleash -S l` inside the package directory, or
  `make mutation` for all of them. Settings live in `.gremlins.yaml`; every
  package must kill more than 80% of its mutants.
- A `LIVED` mutant marks a line where a bug would go unnoticed. Add an
  assertion that fails for it; symmetric test data, such as half the lines
  covered, is the usual cause. Leave a mutant only when it is equivalent (the
  code behaves the same either way) and say which ones in the PR.
- Every bug fix comes with a test that fails without the fix.
- `e2e/` builds the real binary and checks its output against golden files
  for `e2e/testdata/constructs`, a chart with one template per traced
  construct. When you add a construct, add it to that chart, run
  `go test ./e2e -update`, and check every changed line of the golden diff by
  hand. The same package diffs helmcov's render against `helm template`; it
  skips when `helm` is not installed.

## Commits

Commits follow [Conventional Commits](https://www.conventionalcommits.org).
Release Please reads them to pick the next version and write `CHANGELOG.md`.
The `commit-msg` hook and CI both check the format.

```
<type>(<scope>): <summary>

<body: why, not what>

BREAKING CHANGE: <what users must change>
```

| Type | Use for | Release |
|------|---------|---------|
| `feat` | New user-visible behaviour: a flag, a construct, an output field | minor |
| `fix` | A bug users can hit | patch |
| `perf` | Faster with the same output | patch |
| `revert` | Reverting an earlier commit | patch |
| `refactor` | Internal change, same behaviour | none |
| `test` | Tests only | none |
| `docs` | Documentation only | none |
| `ci` | Workflows and CI scripts | none |
| `build` | Build, GoReleaser, Dockerfiles | none |
| `chore` | Anything else, including dependency bumps | none |

Add `!` after the type, or a `BREAKING CHANGE:` footer, when a flag, an action
input, an output format or a trace key changes incompatibly. That cuts a major
release once the project is past 1.0, and a minor one before.

Scopes: `cli`, `instrumentation`, `coverage`, `report`, `chartloader`,
`valuegen`, `action`, `ci`, `release`, `deps`. Leave the scope out when a change
spans several.

The summary is imperative, lowercase and has no full stop:
`fix(instrumentation): track dot inside range bodies`.

Never add `Co-authored-by` trailers for AI tools, or "generated with" lines.

## Pull requests

The PR title becomes the commit on `main`, so it follows the commit rules above.
CI rejects a title that does not.

The body leads with one to three sentences: what changed and why. Then, only
when they apply:

- Breaking changes, including "none" whenever a flag, action input, output
  format or trace key is touched.
- What a user can now do, or what stops going wrong.
- Decisions a reviewer would otherwise ask about.
- Lower-level detail and how it was tested, inside a `<details>` block.

Do not walk through the diff file by file. Link the issue instead of retelling
it.

## Releases

Merging to `main` lets Release Please open or update a release PR. Merging that
PR tags `vX.Y.Z`, publishes binaries and the GHCR image, and moves the `v1` tag
the action uses. See `docs/release-flow.md`.

Do not edit `CHANGELOG.md`, `.release-please-manifest.json` or version strings
by hand.
