# ── Stage 1: dev ──────────────────────────────────────────────────────────────
# Used by docker-compose for local development and the full test suite.
# Provides the Go toolchain, psql, and openssl so run_tests.sh can:
#   - exec `go test ./internal/...` and `go test ./API_TESTS/...`
#   - exec `go run ./cmd/migrate up`
#   - check `test -f /tmp/lms-dev.key` for readiness
# Source code is bind-mounted at /workspace/backend at run time.
FROM golang:1.24 AS dev

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        openssl \
        postgresql-client \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace/backend

# Pre-download module dependencies so go test/run is fast at runtime.
# The actual source is provided by the bind-mount; these files are here
# only to prime the module cache during image build.
COPY go.mod go.sum ./
RUN go mod download

EXPOSE 8080

# ── Stage 2: builder ──────────────────────────────────────────────────────────
# Compiles fully-static production binaries.
FROM golang:1.24 AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /lms-server  ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" \
        -o /lms-migrate ./cmd/migrate

# ── Stage 3: runtime ──────────────────────────────────────────────────────────
# Minimal production image — just the compiled binaries and runtime deps.
FROM debian:bookworm-slim AS runtime

RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        openssl \
        postgresql-client \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /lms-server  ./lms-server
COPY --from=builder /lms-migrate ./lms-migrate

EXPOSE 8080
CMD ["./lms-server"]
