SHELL := /bin/bash

.PHONY: bootstrap build lint test go-build go-test rust-check rust-test web-build web-lint scoring

bootstrap:
	pnpm install
	go work sync
	go work edit -go=1.22
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
	cd crates && cargo build --release -p scoring
	@mkdir -p libs/go-scoring/lib
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); ARCH=$$(uname -m); \
	if [ "$$OS" = "darwin" ]; then \
		cp crates/target/release/libscoring.dylib libs/go-scoring/lib/libscoring.dylib; \
	else \
		cp crates/target/release/libscoring.so libs/go-scoring/lib/libscoring.so; \
	fi
	@echo "Scoring cdylib copied to libs/go-scoring/lib/"

scoring-linux-amd64:
	cd crates && cargo build --release -p scoring --target x86_64-unknown-linux-gnu
	@mkdir -p libs/go-scoring/lib/linux-amd64
	@cp crates/target/x86_64-unknown-linux-gnu/release/libscoring.so libs/go-scoring/lib/linux-amd64/

scoring-linux-arm64:
	cd crates && cargo build --release -p scoring --target aarch64-unknown-linux-gnu
	@mkdir -p libs/go-scoring/lib/linux-arm64
	@cp crates/target/aarch64-unknown-linux-gnu/release/libscoring.so libs/go-scoring/lib/linux-arm64/
