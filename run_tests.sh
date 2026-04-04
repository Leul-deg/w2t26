#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "==> Backend unit tests"
(
  cd "$ROOT_DIR/repo/backend"
  go test ./internal/...
)

echo
echo "==> Frontend type-check"
(
  cd "$ROOT_DIR/repo/frontend"
  npm run lint
)

echo
echo "==> Frontend tests"
(
  cd "$ROOT_DIR/repo/frontend"
  npm run test
)

if [[ -n "${DATABASE_TEST_URL:-}" ]]; then
  echo
  echo "==> Backend integration tests"
  (
    cd "$ROOT_DIR/repo/backend"
    go test ./tests/integration/... -v
  )
else
  echo
  echo "Skipping backend integration tests because DATABASE_TEST_URL is not set."
fi

echo
echo "All requested checks completed."
