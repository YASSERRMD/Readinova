SHELL := /bin/bash

.PHONY: bootstrap build lint test \
        go-build go-test go-lint \
        rust-check rust-test \
        web-build web-lint \
        scoring scoring-linux-amd64 scoring-linux-arm64

# ---------------------------------------------------------------------------
# Developer bootstrap
# ---------------------------------------------------------------------------
bootstrap:
	pnpm install
	go work sync
	pnpm exec lefthook install
	$(MAKE) build

# ---------------------------------------------------------------------------
# Aggregate targets
# ---------------------------------------------------------------------------
build: go-build rust-check web-build

lint: go-lint rust-check web-lint

test: go-test rust-test

# ---------------------------------------------------------------------------
# Go
# ---------------------------------------------------------------------------
go-build:
	go build ./...

go-test:
	cd apps/api && go test ./... -race -count=1
	cd libs/go-scoring && go test ./...

go-lint:
	cd apps/api && go vet ./...
	@if command -v staticcheck >/dev/null 2>&1; then \
		cd apps/api && staticcheck ./...; \
	else \
		echo "staticcheck not installed — skipping (run: go install honnef.co/go/tools/cmd/staticcheck@latest)"; \
	fi

# ---------------------------------------------------------------------------
# Rust
# ---------------------------------------------------------------------------
rust-check:
	cd crates && cargo fmt --all -- --check
	cd crates && cargo clippy --workspace --all-targets -- -D warnings
	cd crates && cargo check --workspace

rust-test:
	cd crates && cargo test --workspace

# ---------------------------------------------------------------------------
# Web
# ---------------------------------------------------------------------------
web-build:
	pnpm --filter @readinova/web build

web-lint:
	pnpm --filter @readinova/web lint
	pnpm --filter @readinova/web exec tsc --noEmit

# ---------------------------------------------------------------------------
# Scoring cdylib (native)
# ---------------------------------------------------------------------------
scoring:
	cd crates && cargo build --release -p scoring
	@mkdir -p libs/go-scoring/lib
	@OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	if [ "$$OS" = "darwin" ]; then \
		cp crates/target/release/libscoring.dylib libs/go-scoring/lib/libscoring.dylib; \
		echo "Copied libscoring.dylib"; \
	else \
		cp crates/target/release/libscoring.so libs/go-scoring/lib/libscoring.so; \
		echo "Copied libscoring.so"; \
	fi

scoring-linux-amd64:
	cd crates && cargo build --release -p scoring --target x86_64-unknown-linux-gnu
	@mkdir -p libs/go-scoring/lib/linux-amd64
	@cp crates/target/x86_64-unknown-linux-gnu/release/libscoring.so libs/go-scoring/lib/linux-amd64/

scoring-linux-arm64:
	cd crates && cargo build --release -p scoring --target aarch64-unknown-linux-gnu
	@mkdir -p libs/go-scoring/lib/linux-arm64
	@cp crates/target/aarch64-unknown-linux-gnu/release/libscoring.so libs/go-scoring/lib/linux-arm64/
