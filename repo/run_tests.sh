#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required to run the packaged test suite."
  exit 1
fi

wait_for_backend_exec() {
  local attempts=0
  until docker compose exec -T backend bash -lc "command -v psql >/dev/null 2>&1 && test -f /tmp/lms-dev.key" >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if (( attempts > 120 )); then
      echo "Backend container did not become ready for exec commands in time."
      exit 1
    fi
    sleep 1
  done
}

echo "==> Resetting Dockerized environment"
(
  cd "$ROOT_DIR"
  docker compose down -v --remove-orphans >/dev/null 2>&1 || true
)

echo "==> Ensuring Dockerized test environment"
(
  cd "$ROOT_DIR"
  docker compose up -d postgres backend frontend >/dev/null
)

DB_HOST_PORT="$(cd "$ROOT_DIR" && docker compose port postgres 5432 | awk -F: 'END {print $NF}')"
if command -v pg_isready >/dev/null 2>&1; then
  until pg_isready -h localhost -p "$DB_HOST_PORT" >/dev/null 2>&1; do
    sleep 1
  done
fi

wait_for_backend_exec

echo "==> Resetting test database"
(
  cd "$ROOT_DIR"
  docker compose exec -T postgres bash -lc "
    export PGPASSWORD='changeme'
    psql -U lms_user -d postgres -c \"DROP DATABASE IF EXISTS lms_test WITH (FORCE);\"
    psql -U lms_user -d postgres -c \"CREATE DATABASE lms_test OWNER lms_user;\"
  "
  docker compose exec -T backend bash -lc "
    cd /workspace/backend &&
    export DATABASE_URL='postgresql://lms_user:changeme@postgres:5432/lms_test?sslmode=disable' &&
    export MIGRATIONS_PATH='/workspace/migrations' &&
    /usr/local/go/bin/go run ./cmd/migrate up >/dev/null
  "
)

echo
echo "==> Backend unit tests"
(
  cd "$ROOT_DIR"
  docker compose exec -T backend bash -lc "
    cd /workspace/backend &&
    /usr/local/go/bin/go test ./internal/...
  "
)

echo
echo "==> Frontend type-check"
(
  cd "$ROOT_DIR"
  docker compose exec -T frontend bash -lc "
    cd /workspace/frontend &&
    npm run lint
  "
)

echo
echo "==> Frontend tests"
(
  cd "$ROOT_DIR"
  docker compose exec -T frontend bash -lc "
    cd /workspace/frontend &&
    npm run test
  "
)

echo
echo "==> Backend integration tests"
(
  cd "$ROOT_DIR"
  docker compose exec -T backend bash -lc "
    cd /workspace/backend &&
    export DATABASE_TEST_URL='postgresql://lms_user:changeme@postgres:5432/lms_test?sslmode=disable' &&
    /usr/local/go/bin/go test ./tests/integration/... -v
  "
)

echo
echo "All requested checks completed."
