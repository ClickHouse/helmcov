# Contributing to helmcov

Bug reports, fixes and new template constructs are welcome. For anything larger
than a bug fix, open an issue first so the approach can be agreed before you
write the code.

## Setting up

You need Go (the version in `go.mod` or newer) and, for linting,
[golangci-lint](https://golangci-lint.run).

```bash
git clone https://github.com/ClickHouse/helmcov.git
cd helmcov
make setup   # installs the commit-msg hook
make ci      # what CI runs
```

## Making a change

- Start with a failing test. Bug fixes need a test that fails without the fix.
- Keep each pull request to one concern.
- Run `make ci` before pushing.
- `AGENTS.md` has the full conventions: package layout, Go style, coverage trace
  keys and testing rules. They apply to people as well as coding agents.

## Commit messages and PR titles

Use [Conventional Commits](https://www.conventionalcommits.org), for example
`fix(instrumentation): track dot inside range bodies`. The type decides the next
release: `feat` is a minor bump, `fix` a patch, and `docs`, `test`, `ci` or
`chore` release nothing. Mark breaking changes with `!` or a `BREAKING CHANGE:`
footer. The PR title is checked in CI because it becomes the commit on `main`.

In the PR description, say what changed and why in a few sentences, and say
whether any flag, action input or output format changes.

## Reporting bugs

Open an issue with the helmcov version, the command you ran, and the smallest
chart that reproduces the problem. For coverage bugs, include the template, the
values and the trace key you expected to see.

Report security problems privately, as described in `SECURITY.md`.

## Licence

helmcov is licensed under Apache-2.0. By contributing you agree that your
contribution is licensed under the same terms.
