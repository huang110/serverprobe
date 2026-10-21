# ServerProbe

<div align="center">

**轻量级、现代化的服务器监控探针系统**

[![CI](https://github.com/huang110/serverprobe/actions/workflows/ci.yml/badge.svg)](https://github.com/huang110/serverprobe/actions/workflows/ci.yml)
[![Docker Build](https://github.com/huang110/serverprobe/actions/workflows/docker.yml/badge.svg)](https://github.com/huang110/serverprobe/actions/workflows/docker.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue)](https://go.dev/)
[![Node Version](https://img.shields.io/badge/node-%3E%3D18-green)](https://nodejs.org/)
[![PRs Welcome](https://img.shields.io/badge/PRs-welcome-brightgreen.svg)](CONTRIBUTING.md)

[English](README.md) | [简体中文](README_zh.md)

</div>

---

## 特性

- 轻量级 Agent：单文件部署，资源占用低
- 实时监控：CPU、内存、磁盘、网络、负载、TCP/UDP 连接
- WebSocket 推送：实时指标、Agent 状态、告警事件
- 告警引擎：阈值、连续触发、恢复通知、Webhook
- 数据聚合与保留：1m → 5m → 1h → 1d
- 安全：HMAC-SHA256 签名、JWT、Token 轮换与黑名单
- 部署：Docker Compose、systemd、Nginx

## 架构

```text
浏览器（Vue 3 + ECharts）
   │ HTTPS / WebSocket
   ▼
Nginx（前端）
   │ /api、/ws
   ▼
Go Server（Gin + GORM + Redis + WebSocket）
   │                     │
   │ gRPC                │ MySQL / Redis
   ▼                     ▼
Go Agent            MySQL 8 + Redis 7
```

## 快速开始

### Docker（推荐）

```bash
git clone https://github.com/huang110/serverprobe.git
cd serverprobe
cp .env.example .env
# 修改 .env 中的密钥
./scripts/start.sh
```

访问 <http://localhost>，使用 `admin / admin123` 登录。

> 对外暴露前请修改所有默认密码和密钥。

### 手动部署

参见 [部署文档](docs/DEPLOYMENT.md)。

## 项目结构

```text
serverprobe/
├── agent/          # Go Agent
├── server/         # Go 后端
├── frontend/       # Vue 3 监控大屏
├── tests/          # 端到端集成测试
├── proto/          # protobuf 定义
├── scripts/        # 启动、安装、备份、测试脚本
├── docs/           # 部署、API、数据库 schema
├── grafana/        # 可选 Grafana 仪表板
├── docker-compose.yml
└── agent/、server/ 下的 Makefile
```

## 技术栈

| 组件 | 技术 |
|---|---|
| Agent | Go 1.21+、gRPC、gopsutil |
| Server | Go 1.21+、Gin、GORM、Redis、MySQL |
| Frontend | Vue 3、TypeScript、Vite、TailwindCSS、ECharts、Pinia |
| 部署 | Docker Compose、systemd、Nginx |

## 测试

```bash
# 环境检查
./scripts/setup-test-env.sh

# 单元测试 + 集成测试 + 前端构建
./scripts/run-integration-tests.sh
```

集成测试会启动真实 Server 进程，使用临时 SQLite + miniredis，覆盖注册、心跳、实时缓存、聚合、告警、WebSocket 和并发场景。

前端 E2E：

```bash
cd frontend
npx playwright install chromium
npm run test:e2e
```

## 文档

- [部署指南](docs/DEPLOYMENT.md)
- [HTTP / WebSocket API](docs/API.md)
- [数据库 schema](docs/schema.sql)
- [贡献指南](CONTRIBUTING.md)
- [变更日志](CHANGELOG.md)

## 已知限制

- 自定义脚本指标尚未实现
- 暂无独立离线检测任务，WebSocket 主要推送 `online` 状态
- Token 轮换幂等缓存尚未实现
- `GetConfigResponse` / `RotateTokenResponse` 协议缺少 `message` 字段
- gRPC Server 未直接启用 TLS，生产环境建议部署在私网或 TLS 代理之后
- Docker Compose 与 Playwright 浏览器测试未在准备环境中实际执行

## 开源协议

[MIT](LICENSE)
