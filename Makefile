SHELL := /bin/bash

.PHONY: help setup hooks tidy fmt fmt-check lint test integration-test build ci release-check

help:
	@echo "Targets:"
	@echo "  make setup            # install hooks and tidy modules"
	@echo "  make hooks            # install git commit-msg hook"
	@echo "  make tidy             # go mod tidy"
	@echo "  make fmt              # format Go code"
	@echo "  make fmt-check        # fail if Go code is not formatted"
	@echo "  make lint             # go vet + optional golangci-lint"
	@echo "  make test             # run all Go tests"
	@echo "  make integration-test # run integration-oriented CLI tests"
	@echo "  make build            # build CLI binary"
	@echo "  make ci               # fmt-check + lint + test + integration-test + build"
	@echo "  make release-check    # ci + goreleaser config validation (if installed)"

setup: hooks tidy

hooks:
	./scripts/install-hooks.sh

tidy:
	go mod tidy

fmt:
	gofmt -w .

fmt-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		echo "The following files are not gofmt formatted:"; \
		echo "$$files"; \
		exit 1; \
	fi

lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run --timeout=5m; \
	elif [ "$${CI:-}" = "true" ]; then \
		echo "golangci-lint is required in CI but was not found" >&2; \
		exit 1; \
	else \
		echo "golangci-lint not found; skipping local golangci-lint run"; \
	fi

test:
	go test ./...

integration-test:
	go test ./internal/cli -run 'TestRunAgainstExamples|TestRunAgainstMonorepoExamples' -count=1 -v

build:
	go build ./cmd/helmcov

ci: fmt-check lint test integration-test build

release-check: ci
	@if command -v goreleaser >/dev/null 2>&1; then \
		goreleaser check; \
	else \
		echo "goreleaser not found; skipping goreleaser check"; \
	fi
