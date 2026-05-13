# Readinova AI Readiness Platform

Readinova is a multi-tenant AI readiness assessment platform. The planned stack is Go services, a deterministic Rust scoring core exposed to Go through FFI, a React and Tailwind frontend, PostgreSQL for tenant data, and ClickHouse for analytics.

This repository is currently in Phase 01 of the platform plan: repository foundation, workspace tooling, CI, and commit hygiene.

## Repository Layout

```text
apps/api          Go services
apps/web          React, Vite, and Tailwind frontend
crates/scoring    Rust scoring core
libs/go-scoring   Go wrapper for the Rust scoring library
migrations        PostgreSQL migrations
infra             Docker Compose, Kubernetes, and Terraform assets
docs              Architecture decision records
scripts           Development scripts
```

## Prerequisites

- Go 1.22 or newer
- Rust stable with `rustfmt` and `clippy`
- Node.js 20 LTS or newer
- pnpm 10 or newer

## Quick Start

```bash
make bootstrap
```

The bootstrap target installs web dependencies, syncs the Go workspace, installs Lefthook hooks, and runs the current build checks.

Useful targets:

```bash
make build
make lint
make test
make scoring
```

## Commit Convention

Commits must use Conventional Commits:

```text
feat(scope): add capability
fix(scope): correct behavior
chore(scope): maintain tooling
docs(scope): update documentation
test(scope): add coverage
ci(scope): update pipeline
```

Lefthook runs formatting and lint checks before commits. Commitlint validates commit messages.
