# Stage 1: Build frontend assets
FROM oven/bun:1.3.8 AS frontend

WORKDIR /build

COPY ui/package.json ui/bun.lock ui/
RUN cd ui && bun install --frozen-lockfile

COPY admin/package.json admin/bun.lock admin/
RUN cd admin && bun install --frozen-lockfile

COPY ui/ ui/
RUN cd ui && bun run build

COPY admin/ admin/
RUN cd admin && bun run build

# Stage 2: Build Go binary
FROM golang:1.26.1 AS backend

WORKDIR /build

COPY api/go.mod api/go.sum* api/
RUN cd api && go mod download

COPY api/ api/
COPY --from=frontend /build/ui/dist api/ui/dist
COPY --from=frontend /build/admin/dist api/admin/dist

RUN cd api && CGO_ENABLED=1 go build -ldflags="-s -w" -o /standalone .

# Stage 3: Minimal runtime
FROM debian:bookworm-slim

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=backend /standalone /usr/local/bin/standalone

EXPOSE 23710

ENTRYPOINT ["standalone"]
