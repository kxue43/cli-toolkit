.DEFAULT_GOAL := all

export PKG_CONFIG_PATH=$(CURDIR)/git2go/static-build/install/lib/pkgconfig

test:
	@go test ./...
.PHONY: test

coverage:
	@go test -coverprofile=coverage.out ./...
.PHONY: coverage

show: coverage
	@go tool cover -html=coverage.out
.PHONY: show

lint:
	@pre-commit run --all-files golangci-lint-full
.PHONY: lint

verify:
	@pre-commit run --all-files golangci-lint-verify
.PHONY: verify

fmt:
	@pre-commit run --all-files golangci-lint-fmt
.PHONY: fmt

lint-all:
	@pre-commit run --all-files
.PHONY: lint-all

all: test lint
.PHONY: all

toolkit-git-hook:
	@go build -tags static ./cmd/toolkit-git-hook
.PHONY: toolkit-git-hook

try-run-hook: toolkit-git-hook
	@./toolkit-git-hook
.PHONY: test-run-hook
