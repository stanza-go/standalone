# Stanza Standalone

[![CI](https://github.com/stanza-go/standalone/actions/workflows/ci.yml/badge.svg)](https://github.com/stanza-go/standalone/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A fully built, production-ready application powered by [Stanza Framework](https://github.com/stanza-go/framework). Fork this repo and build your idea.

**One binary. One SQLite file. One data dir. 4 hours to production.**

## What's Included

| Project | Dev Port | Stack |
|---------|----------|-------|
| `api/` | 23710 | Go + Stanza Framework |
| `admin/` | 23706 | Vite + Bun + React + Mantine |
| `ui/` | 23700 | Vite + Bun (blank canvas) |

### API (33 modules)

Auth (JWT + refresh tokens), admin panel endpoints, user management, API keys, webhooks, job queue, cron scheduler, file uploads, notifications, audit log, dashboard, settings, password reset, Prometheus metrics, health checks.

### Admin Panel (20 pages)

Dashboard, admins, users, roles, sessions, API keys, webhooks, queue, cron, logs, database, settings, audit, notifications, uploads, profile — built with Mantine UI.

### UI

A blank HTML file. Your idea decides what it becomes — SPA, landing page, or nothing at all.

## Quick Start

Requires Go 1.26.1+, Bun, and CGo.

```bash
# Development — 3 processes with hot reload
make dev

# Production build — single binary with embedded frontends
make build
./api/bin/standalone
```

## Production

The build produces a single binary (~10MB) that serves everything:

| Path | Serves |
|------|--------|
| `/*` | Embedded UI |
| `/admin/*` | Embedded admin panel |
| `/api/*` | Go API handlers |

### Deploy

```bash
# Docker
make docker
docker run -v /data:/data -p 8080:8080 stanza

# Railway
railway up
```

### Data Directory

All state lives in one directory (default `~/.stanza/`, override with `DATA_DIR`):

```
~/.stanza/
├── database.sqlite
├── logs/
├── uploads/
├── backups/
└── config.yaml
```

### Ops

| Action | Command |
|--------|---------|
| Backup | `stanza export` |
| Restore | `stanza import backup.zip` |
| Migrate | Copy data dir to new machine |
| Update | Replace binary, restart |

## Documentation

See [stanza-go/docs](https://github.com/stanza-go/docs) for framework reference and recipes.

## License

MIT
