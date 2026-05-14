![Readinova Hero Banner](docs/assets/hero-banner.png)

<div align="center">

# Readinova

**AI Readiness Assessment Platform**

_Measure, validate, and accelerate your organisation's AI readiness — from self-assessment to evidence-backed scoring._

[![CI](https://github.com/YASSERRMD/Readinova/actions/workflows/ci.yml/badge.svg)](https://github.com/YASSERRMD/Readinova/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/Go-1.22%2B-00ADD8?logo=go)](https://go.dev)
[![Rust](https://img.shields.io/badge/Rust-stable-CE422B?logo=rust)](https://www.rust-lang.org)
[![React](https://img.shields.io/badge/React-18-61DAFB?logo=react)](https://react.dev)
[![License](https://img.shields.io/badge/license-MIT-6366f1)](LICENSE)

</div>

---

## Overview

Readinova is a multi-tenant SaaS platform that guides organisations through a structured AI readiness assessment, produces a composite score across five dimensions, and generates evidence-backed recommendations. It combines a Rust-powered deterministic scoring engine with a Go REST API, a React/Vite/Tailwind frontend, and a Stripe-integrated billing layer.

### Key capabilities

| Capability | Description |
|---|---|
| **5-Dimension Assessment** | Strategy, Data Governance, Technology, Talent & Culture, Ethics & Governance |
| **Perception Gap Engine** | Layer B evidence score from connectors, master composite = 0.4S + 0.5E + 0.1(100−\|P\|) |
| **Evidence Connectors** | Pluggable `Connector` interface; ships with `TestConnector` and `AzureConnector` (ARM + Graph) |
| **Recommendation Engine** | YAML template library, priority-ranked, wave-grouped (Act Now / Next Quarter / Roadmap) |
| **Signed Audit Artefacts** | Ed25519 signed scoring snapshots, offline-verifiable via `readiness-verify` CLI |
| **PDF Reports** | Chromium-rendered PDF with watermark control |
| **OpenTelemetry + Prometheus** | HTTP trace middleware, `/metrics` endpoint, k6 smoke tests |

---

## Architecture

![Readinova Architecture](docs/assets/architecture.png)

### Repository layout

```
apps/
  api/                  Go REST API (net/http, pgx, stripe-go)
    cmd/
      readiness-verify/ Ed25519 artefact verification CLI
      seedframework/    Framework YAML seed tool
    internal/
      artefact/         Ed25519 sign + verify
      billing/          Tier limits and Stripe helpers
      connector/        Connector interface, TestConnector, AzureConnector
      httpapi/          HTTP handlers and middleware
      perception/       Layer B aggregation and gap engine
      platform/
        telemetry/      OpenTelemetry + Prometheus middleware
      recommend/        Recommendation engine + YAML templates
      report/           HTML/PDF report rendering (chromedp)
      scoring/          CGo FFI bridge to Rust scoring lib
  web/                  React + Vite + Tailwind SPA
    src/
      api/              Axios API client modules
      components/       RadarChart, DimensionCard, RubricCard, …
      contexts/         AuthContext (JWT + refresh timer)
      pages/            Login, Assessments, Questionnaire, Dashboard, …

crates/
  scoring/              Rust scoring engine (cdylib FFI)

libs/
  go-scoring/           Go CGo wrapper for libscoring.so

migrations/             goose SQL migrations (00001 → 00018)
tests/
  k6/                   k6 smoke test scripts
docs/
  assets/               Images and architecture diagrams
```

---

## Getting started

### Prerequisites

| Tool | Version |
|---|---|
| Go | 1.22+ |
| Rust | stable |
| Node.js | 20 LTS |
| pnpm | 10+ |
| PostgreSQL | 16 |

### Bootstrap

```bash
# Clone and enter the repo
git clone https://github.com/YASSERRMD/Readinova.git
cd Readinova

# Install all dependencies and set up git hooks
make bootstrap
```

### Run in development

```bash
# 1. Start PostgreSQL (Docker)
docker compose up -d postgres

# 2. Apply migrations
DATABASE_URL=postgres://readinova:readinova@localhost:5432/readinova \
  goose -dir migrations postgres "$DATABASE_URL" up

# 3. Start the API
cd apps/api
READINOVA_DATABASE_URL=postgres://... \
JWT_SECRET=dev-secret \
  go run .

# 4. Start the frontend (separate terminal)
cd apps/web
pnpm dev
```

The API listens on `http://localhost:8080` and the frontend on `http://localhost:5173`.

---

## Assessment flow

```
Create Assessment → Set Role Assignments → Start → Answer Questions
        → Submit → Score (Rust engine) → Perception Gap → Recommendations
        → Download PDF Report → Sign Audit Artefact
```

1. **Create** — Owner creates an assessment tied to the AI readiness framework.
2. **Assign** — Map each question's target role to a team member.
3. **Answer** — Each assignee selects a rubric level (1–5) per question with optional free text.
4. **Score** — `POST /v1/assessments/{id}/score` calls the Rust engine via CGo FFI, computes dimension and composite scores, persists the run.
5. **Evidence** — Sync evidence connectors; `POST /v1/assessments/{id}/perception-gap` computes Layer B and master composite.
6. **Recommend** — `GET /v1/assessments/{id}/recommendations` returns wave-grouped actions from the YAML template library.
7. **Report** — `GET /v1/assessments/{id}/report?format=pdf` renders a Chromium PDF.
8. **Artefact** — `POST /v1/assessments/{id}/artefacts` signs the result with Ed25519 for audit trail.

---

## Scoring model

The composite score is computed by the Rust engine in three layers:

```
Layer A  — Self-assessment composite (0–100)
           Dimension scores → weighted aggregate

Layer B  — Evidence composite (0–100)
           Connector signals normalised and averaged per dimension

Master   = 0.4 × LayerA + 0.5 × LayerB + 0.1 × (100 − |LayerA − LayerB|)
```

Derived indices (Readiness Index, Governance Risk, Execution Capacity, Value Realisation) are computed from dimension score sub-sets.

---

## Evidence connectors

Implement the `Connector` interface to add any data source:

```go
type Connector interface {
    Type() string
    Connect(ctx context.Context, credentials map[string]any) error
    Collect(ctx context.Context, dimensions []string) ([]Signal, error)
    Disconnect(ctx context.Context) error
}
```

Built-in connectors:

| Connector | Signals collected |
|---|---|
| `test` | Synthetic deterministic signals for dev |
| `azure` | ARM: subscription count, policy compliance · Graph: users, groups, SPs, conditional access |

---

## Audit artefacts

Each signed artefact is self-contained and can be verified offline:

```bash
# Export an artefact from the API and verify it
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/v1/assessments/{id}/artefacts \
  | jq '.[0]' > artefact.json

readiness-verify -f artefact.json
# VALID
#   Assessment:      <uuid>
#   Composite Score: 72.50
#   Signed At:       2026-05-14 21:00:00 UTC
#   Payload Hash:    a3f9...
```

---

## Observability

| Signal | Endpoint / Source |
|---|---|
| Prometheus metrics | `GET /metrics` |
| HTTP traces | OpenTelemetry SDK (swap exporter for OTLP in prod) |
| `http_requests_total` | Counter by method, path, status |
| `http_request_duration_seconds` | Histogram (p50, p95, p99) |

Run the k6 smoke test:

```bash
k6 run tests/k6/smoke.js -e BASE_URL=http://localhost:8080
```

---

## CI pipeline

| Job | Checks |
|---|---|
| **Rust** | `cargo fmt`, `cargo clippy -D warnings`, `cargo test`, build `libscoring.so` |
| **Go** | `go vet`, `go build`, `go test -race -count=1` (matrix: Go 1.22, 1.23) |
| **Web** | `pnpm lint`, `pnpm build`, `tsc --noEmit` |
| **Trivy** | Filesystem scan, CRITICAL+HIGH, SARIF → GitHub Security |
| **govulncheck** | Advisory scan on Go modules |

---

## Development commands

```bash
make build          # Build all targets
make lint           # Run all linters (Go, Rust, ESLint)
make test           # Run all test suites
make scoring        # Build the Rust scoring cdylib

# Individual
cd apps/api && go test ./...
cd apps/web && pnpm lint
cd crates && cargo test --workspace
```

---

## Commit convention

This repository uses [Conventional Commits](https://www.conventionalcommits.org):

```
feat(scope):  new capability
fix(scope):   bug correction
chore(scope): tooling maintenance
docs(scope):  documentation update
test(scope):  add or update tests
ci(scope):    pipeline changes
```

Lefthook enforces formatting and lint checks pre-commit. Commitlint validates commit messages.

---

## License

MIT — see [LICENSE](LICENSE).

---

<div align="center">
Built with Go · Rust · React · PostgreSQL
</div>
