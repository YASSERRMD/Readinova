#!/usr/bin/env bash
set -euo pipefail

docker compose -f infra/docker-compose.yml up -d postgres

export READINOVA_DATABASE_URL="${READINOVA_DATABASE_URL:-postgres://readinova:readinova@localhost:54329/readinova_test?sslmode=disable}"

go test ./apps/api/internal/db/...

