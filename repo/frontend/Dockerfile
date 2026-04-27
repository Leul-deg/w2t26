# ── Stage 1: dev ──────────────────────────────────────────────────────────────
# Used by docker-compose for local development and the full test suite.
# Uses node:20 (Debian-based, has bash) so run_tests.sh can bash -lc exec.
# node_modules are installed here so Docker's anonymous-volume mechanism
# can preserve them when the source bind-mount is applied at run time:
#   volumes:
#     - ./frontend:/workspace/frontend        ← overrides source
#     - /workspace/frontend/node_modules      ← anonymous vol keeps these deps
FROM node:20 AS dev

WORKDIR /workspace/frontend

COPY package.json package-lock.json ./
RUN npm ci

EXPOSE 3000

# ── Stage 2: builder ──────────────────────────────────────────────────────────
# Builds the production Vite bundle.
FROM node:20-alpine AS builder

WORKDIR /app

COPY package.json package-lock.json ./
RUN npm ci

COPY . .

ARG VITE_API_BASE_URL=http://localhost:8080
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL

RUN npm run build

# ── Stage 3: runtime ──────────────────────────────────────────────────────────
# Serves the built static bundle via vite preview.
FROM node:20-alpine AS runtime

WORKDIR /app

COPY --from=builder /app/dist          ./dist
COPY --from=builder /app/package.json  ./package.json
COPY --from=builder /app/node_modules  ./node_modules
COPY --from=builder /app/vite.config.ts ./vite.config.ts

EXPOSE 3000
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0", "--port", "3000"]
