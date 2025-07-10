lint:
	pre-commit run --all-files golangci-lint-full
.PHONY: lint

all:
	pre-commit run --all-files
.PHONY: all

verify:
	pre-commit run --all-files golangci-lint-verify
.PHONY: verify

fmt:
	pre-commit run --all-files golangci-lint-fmt
.PHONY: fmt

tartufo:
	pre-commit run tartufo
