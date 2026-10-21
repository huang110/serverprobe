# ServerProbe

<div align="center">

**Lightweight, modern server monitoring system**

[![CI](https://github.com/huang110/serverprobe/actions/workflows/ci.yml/badge.svg)](https://github.com/huang110/serverprobe/actions/workflows/ci.yml)
[![Docker Build](https://github.com/huang110/serverprobe/actions/workflows/docker.yml/badge.svg)](https://github.com/huang110/serverprobe/actions/workflows/docker.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://go.dev/)
[![Node Version](https://img.shields.io/badge/node-%3E%3D18-green)](https://nodejs.org/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[English](README.md) | [简体中文](README_zh.md)

</div>

---

## Features

- Lightweight Agent: single binary, low resource usage
- Real-time metrics: CPU, memory, disk, network, load, TCP/UDP connections
- WebSocket push: realtime metrics, agent status, alerts
- Alert engine: thresholds, consecutive triggers, recovery, webhooks
- Data aggregation and retention: 1m → 5m → 1h → 1d
- Security: HMAC-SHA256 signatures, JWT, token rotation and blacklist
- Deployment: Docker Compose, systemd, Nginx

## Architecture

```text
Browser (Vue 3 + ECharts)
   │ HTTPS / WebSocket
   ▼
Nginx (frontend)
   │ /api, /ws
   ▼
Go Server (Gin + GORM + Redis + WebSocket)
   │                     │
   │ gRPC                │ MySQL / Redis
   ▼                     ▼
Go Agent            MySQL 8 + Redis 7
```

## Quick Start

### Docker (recommended)

```bash
git clone https://github.com/huang110/serverprobe.git
cd serverprobe
cp .env.example .env
# edit secrets in .env
./scripts/start.sh
```

Open <http://localhost> and log in with `admin / admin123`.

> Change all default passwords and secrets before exposing the service.

### Manual

See [Deployment Guide](docs/DEPLOYMENT.md).

## Project Structure

```text
serverprobe/
├── agent/          # Go Agent
├── server/         # Go backend
├── frontend/       # Vue 3 dashboard
├── tests/          # end-to-end integration tests
├── proto/          # protobuf definitions
├── scripts/        # start/install/backup/test scripts
├── docs/           # deployment, API, schema
├── grafana/        # optional dashboard
├── docker-compose.yml
└── Makefile targets in agent/ and server/
```

## Technology Stack

| Component | Stack |
|---|---|
| Agent | Go 1.21+, gRPC, gopsutil |
| Server | Go 1.21+, Gin, GORM, Redis, MySQL |
| Frontend | Vue 3, TypeScript, Vite, TailwindCSS, ECharts, Pinia |
| Deployment | Docker Compose, systemd, Nginx |

## Testing

```bash
# environment check
./scripts/setup-test-env.sh

# unit + integration + frontend build
./scripts/run-integration-tests.sh
```

Integration tests launch a real Server process with temporary SQLite and miniredis and cover registration, heartbeat, realtime cache, aggregation, alerts, WebSocket and concurrency.

Frontend E2E:

```bash
cd frontend
npx playwright install chromium
npm run test:e2e
```

## Documentation

- [Deployment Guide](docs/DEPLOYMENT.md)
- [HTTP / WebSocket API](docs/API.md)
- [Database schema](docs/schema.sql)
- [Contributing](CONTRIBUTING.md)
- [Changelog](CHANGELOG.md)

## Known Limitations

- Custom script metrics are not implemented yet.
- No dedicated offline-detection job yet; WebSocket mainly pushes `online` status.
- Token rotation idempotency cache is not implemented.
- `GetConfigResponse` and `RotateTokenResponse` have no `message` field in the proto.
- gRPC TLS is not enabled directly in the server; deploy behind a private network or TLS proxy.
- Docker Compose and Playwright browser tests were not executed in the preparation environment.

## License

[MIT](LICENSE)
