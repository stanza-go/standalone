# Multi-stage Dockerfile for Stanza standalone application.
#
# Build context: the project root (where this Dockerfile lives).
# The framework is fetched from the Go module proxy (github.com/stanza-go/framework).
#
# Build:
#   docker build -t stanza .
#
# Run:
#   docker run -p 23710:23710 -v stanza-data:/data stanza

# ---------------------------------------------------------------------------
# Stage 1: Build frontend assets
# ---------------------------------------------------------------------------
FROM oven/bun:1 AS frontend

WORKDIR /build

# Install UI dependencies
COPY ui/package.json ui/bun.lock ui/
RUN cd ui && bun install --frozen-lockfile

# Install admin dependencies
COPY admin/package.json admin/bun.lock admin/
RUN cd admin && bun install --frozen-lockfile

# Build UI
COPY ui/ ui/
RUN cd ui && bun run build

# Build admin (ARG busts Docker cache when source changes)
ARG CACHE_BUST_ADMIN=1
COPY admin/ admin/
RUN cd admin && bun run build

# ---------------------------------------------------------------------------
# Stage 2: Build Go binary with embedded frontend assets (Alpine for musl)
# ---------------------------------------------------------------------------
FROM golang:1.26.1-alpine AS backend

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

# Download Go dependencies (framework fetched from Go module proxy)
COPY api/go.mod api/go.sum* api/
RUN cd api && go mod download

# Copy application source
COPY api/ api/

# Copy built frontend assets into embed directories
COPY --from=frontend /build/ui/dist api/ui/dist
COPY --from=frontend /build/admin/dist api/admin/dist

# Build metadata — explicit args or auto-detected from Railway env.
# Railway injects RAILWAY_GIT_COMMIT_SHA automatically during builds.
ARG BUILD_VERSION=dev
ARG BUILD_COMMIT=unknown
ARG BUILD_TIME=unknown
ARG RAILWAY_GIT_COMMIT_SHA

# Build binary with CGO for SQLite
RUN COMMIT="${BUILD_COMMIT}"; \
    if [ "${RAILWAY_GIT_COMMIT_SHA}" != "" ]; then \
        COMMIT=$(echo "${RAILWAY_GIT_COMMIT_SHA}" | cut -c1-7); \
    fi; \
    BUILD_TS="${BUILD_TIME}"; \
    if [ "${BUILD_TS}" = "unknown" ]; then \
        BUILD_TS=$(date -u +%Y-%m-%dT%H:%M:%SZ); \
    fi; \
    cd api && GOWORK=off CGO_ENABLED=1 go build -tags prod \
        -ldflags="-s -w \
            -X main.version=${BUILD_VERSION} \
            -X main.commit=${COMMIT} \
            -X main.buildTime=${BUILD_TS}" \
        -o /standalone .

# ---------------------------------------------------------------------------
# Stage 3: Minimal Alpine runtime (~7MB base vs ~74MB debian-slim)
# ---------------------------------------------------------------------------
FROM alpine:3.21

RUN apk add --no-cache ca-certificates su-exec \
    && adduser -D -s /sbin/nologin stanza

COPY --from=backend /standalone /usr/local/bin/standalone

RUN mkdir -p /data && chown stanza:stanza /data

ENV DATA_DIR=/data

EXPOSE 23710

ENTRYPOINT ["sh", "-c", "chown -R stanza:stanza /data && exec su-exec stanza standalone"]
