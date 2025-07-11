test:
	go test ./...
.PHONY: test

lint:
	pre-commit run --all-files golangci-lint-full
.PHONY: lint

verify:
	pre-commit run --all-files golangci-lint-verify
.PHONY: verify

fmt:
	pre-commit run --all-files golangci-lint-fmt
.PHONY: fmt

lint-all:
	pre-commit run --all-files
.PHONY: all

tartufo:
	pre-commit run tartufo
.PHONY: tartufo
