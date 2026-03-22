# Multi-stage Dockerfile for Stanza standalone application.
#
# Build context must be the workspace root (parent of standalone/ and framework/)
# because go.mod uses a replace directive pointing to ../../framework.
#
# Build:
#   docker build -t stanza -f standalone/Dockerfile .
#
# Run:
#   docker run -p 23710:23710 -v stanza-data:/data stanza

# ---------------------------------------------------------------------------
# Stage 1: Build frontend assets
# ---------------------------------------------------------------------------
FROM oven/bun:1 AS frontend

WORKDIR /build

# Install UI dependencies
COPY standalone/ui/package.json standalone/ui/bun.lock standalone/ui/
RUN cd standalone/ui && bun install --frozen-lockfile

# Install admin dependencies
COPY standalone/admin/package.json standalone/admin/bun.lock standalone/admin/
RUN cd standalone/admin && bun install --frozen-lockfile

# Build UI
COPY standalone/ui/ standalone/ui/
RUN cd standalone/ui && bun run build

# Build admin (ARG busts Docker cache when source changes)
ARG CACHE_BUST_ADMIN=1
COPY standalone/admin/ standalone/admin/
RUN cd standalone/admin && bun run build

# ---------------------------------------------------------------------------
# Stage 2: Build Go binary with embedded frontend assets (Alpine for musl)
# ---------------------------------------------------------------------------
FROM golang:1.26.1-alpine AS backend

RUN apk add --no-cache gcc musl-dev

WORKDIR /build

# Copy framework source (required by replace directive in go.mod)
COPY framework/ framework/

# Download Go dependencies
COPY standalone/api/go.mod standalone/api/go.sum* standalone/api/
RUN cd standalone/api && go mod download

# Copy application source
COPY standalone/api/ standalone/api/

# Copy built frontend assets into embed directories
COPY --from=frontend /build/standalone/ui/dist standalone/api/ui/dist
COPY --from=frontend /build/standalone/admin/dist standalone/api/admin/dist

# Build binary with CGO for SQLite
RUN cd standalone/api && CGO_ENABLED=1 go build -tags prod -ldflags="-s -w" -o /standalone .

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
