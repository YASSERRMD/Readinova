SHELL := /bin/bash

.PHONY: bootstrap build lint test go-build go-test rust-check rust-test web-build web-lint scoring

bootstrap:
	pnpm install
	go work sync
	pnpm exec lefthook install
	$(MAKE) build

build: go-build rust-check web-build

lint: rust-check web-lint

test: go-test rust-test

go-build:
	go build ./...
	cd apps/api && go build ./...
	cd libs/go-scoring && go build ./...

go-test:
	cd apps/api && go test ./...
	cd libs/go-scoring && go test ./...

rust-check:
	cd crates && cargo fmt --all -- --check
	cd crates && cargo clippy --workspace --all-targets -- -D warnings
	cd crates && cargo check --workspace

rust-test:
	cd crates && cargo test --workspace

web-build:
	pnpm --filter @readinova/web build

web-lint:
	pnpm --filter @readinova/web lint

scoring:
	cd crates && cargo build -p scoring

