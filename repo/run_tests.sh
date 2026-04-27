#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required to run the packaged test suite."
  exit 1
fi

GO="/usr/local/go/bin/go"

# ── Readiness helpers ──────────────────────────────────────────────────────────

# Wait until the backend container accepts exec commands AND the dev crypto key
# has been written (signals the container startup command completed).
wait_for_backend_exec() {
  local attempts=0
  until docker compose exec -T backend bash -lc \
      "command -v psql >/dev/null 2>&1 && test -f /tmp/lms-dev.key" \
      >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if (( attempts > 120 )); then
      echo "Backend container did not become ready in time."
      docker compose logs backend | tail -30
      exit 1
    fi
    sleep 1
  done
}

# Wait until the frontend container's node_modules are populated
# (Docker's anonymous-volume initialisation from the image layer completes).
wait_for_frontend_exec() {
  local attempts=0
  until docker compose exec -T frontend bash -lc \
      "test -d /workspace/frontend/node_modules/.bin" \
      >/dev/null 2>&1; do
    attempts=$((attempts + 1))
    if (( attempts > 120 )); then
      echo "Frontend container did not become ready in time."
      docker compose logs frontend | tail -30
      exit 1
    fi
    sleep 1
  done
}

# ── Environment ────────────────────────────────────────────────────────────────

echo "==> Resetting Dockerized environment"
(
  cd "$ROOT_DIR"
  docker compose down -v --remove-orphans >/dev/null 2>&1 || true
)

echo "==> Building and starting Dockerized test environment"
(
  cd "$ROOT_DIR"
  docker compose up -d --build postgres backend frontend
)

DB_HOST_PORT="$(cd "$ROOT_DIR" && docker compose port postgres 5432 | awk -F: 'END {print $NF}')"
if command -v pg_isready >/dev/null 2>&1; then
  until pg_isready -h localhost -p "$DB_HOST_PORT" >/dev/null 2>&1; do
    sleep 1
  done
fi

wait_for_backend_exec

# ── Test database ──────────────────────────────────────────────────────────────

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
    $GO run ./cmd/migrate up >/dev/null
  "
)

# ── Backend unit tests ─────────────────────────────────────────────────────────

echo
echo "==> Backend unit tests"
(
  cd "$ROOT_DIR"
  docker compose exec -T backend bash -lc "
    cd /workspace/backend &&
    $GO test ./internal/...
  "
)

# ── Frontend ──────────────────────────────────────────────────────────────────

wait_for_frontend_exec

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

# ── Backend API tests ──────────────────────────────────────────────────────────

echo
echo "==> Backend API tests"
(
  cd "$ROOT_DIR"
  docker compose exec -T backend bash -lc "
    cd /workspace/backend &&
    export DATABASE_TEST_URL='postgresql://lms_user:changeme@postgres:5432/lms_test?sslmode=disable' &&
    $GO test ./API_TESTS/... -v
  "
)

echo
echo "All requested checks completed."
