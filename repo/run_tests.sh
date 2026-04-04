#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
FRONTEND_DIR="$ROOT_DIR/frontend"

if [[ ! -f "$BACKEND_DIR/.env" ]]; then
  cp "$BACKEND_DIR/.env.example" "$BACKEND_DIR/.env"
fi

if [[ ! -f "$FRONTEND_DIR/.env" ]]; then
  cp "$FRONTEND_DIR/.env.example" "$FRONTEND_DIR/.env"
fi

if command -v docker >/dev/null 2>&1; then
  echo "==> Ensuring Dockerized test database"
  (
    cd "$ROOT_DIR"
    docker compose up -d postgres >/dev/null
  )

  DB_HOST_PORT="$(cd "$ROOT_DIR" && docker compose port postgres 5432 | awk -F: 'END {print $NF}')"
  export DATABASE_URL="postgres://lms_user:changeme@localhost:${DB_HOST_PORT}/lms?sslmode=disable"
  export DATABASE_TEST_URL="postgres://lms_user:changeme@localhost:${DB_HOST_PORT}/lms_test?sslmode=disable"
  export MIGRATIONS_PATH="$ROOT_DIR/migrations"

  if command -v pg_isready >/dev/null 2>&1; then
    until pg_isready -h localhost -p "$DB_HOST_PORT" >/dev/null 2>&1; do
      sleep 1
    done
  fi

  if command -v psql >/dev/null 2>&1; then
    PGPASSWORD=changeme psql "$DATABASE_URL" -v ON_ERROR_STOP=1 -c "SELECT 1;" >/dev/null
    PGPASSWORD=changeme psql "postgres://lms_user:changeme@localhost:${DB_HOST_PORT}/postgres?sslmode=disable" -c "DROP DATABASE IF EXISTS lms_test WITH (FORCE);" >/dev/null
    PGPASSWORD=changeme psql "postgres://lms_user:changeme@localhost:${DB_HOST_PORT}/postgres?sslmode=disable" -c "CREATE DATABASE lms_test OWNER lms_user;" >/dev/null
  fi

  (
    cd "$BACKEND_DIR"
    go run ./cmd/migrate up >/dev/null
    DATABASE_URL="$DATABASE_TEST_URL" go run ./cmd/migrate up >/dev/null
  )
fi

echo "==> Backend unit tests"
(
  cd "$BACKEND_DIR"
  go test ./internal/...
)

echo
echo "==> Frontend type-check"
(
  cd "$FRONTEND_DIR"
  npm run lint
)

echo
echo "==> Frontend tests"
(
  cd "$FRONTEND_DIR"
  npm run test
)

if [[ -n "${DATABASE_TEST_URL:-}" ]]; then
  echo
  echo "==> Backend integration tests"
  (
    cd "$BACKEND_DIR"
    go test ./tests/integration/... -v
  )
else
  echo
  echo "Skipping backend integration tests because DATABASE_TEST_URL is not set."
fi

echo
echo "All requested checks completed."
