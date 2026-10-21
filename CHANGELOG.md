# 变更日志

所有重要变更都会记录在此文件中。

格式遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，
版本号遵循 [语义化版本](https://semver.org/lang/zh-CN/)。

## [Unreleased]

### 计划中

- 自定义脚本指标采集
- Agent 离线检测定时任务
- Token 轮换幂等缓存
- 邮件 / Telegram 告警通道
- 多租户支持

---

## [1.0.0] - 2026-10-21

### 新增

- Agent 探针核心功能
  - CPU、内存、磁盘、网络、系统负载、TCP/UDP 连接采集
  - gRPC 通信与 HMAC-SHA256 签名
  - 注册、心跳、Token 轮换
- Server 后端
  - gRPC：注册、心跳、配置拉取、Token 轮换
  - HTTP API：Agent 管理、指标查询、告警规则与记录、登录
  - JWT 鉴权、CORS、限流、请求日志
  - WebSocket 实时推送（realtime_metrics / agent_status / alert）
  - 告警引擎（阈值、连续触发、恢复、Webhook）
  - 数据聚合与清理（1m → 5m → 1h → 1d）
  - 健康检查端点
- Frontend
  - Vue 3 + TypeScript + Vite + TailwindCSS
  - Agent 卡片列表、Agent 详情四个 ECharts 图表
  - 实时数据更新、WebSocket 断线重连
  - 告警规则与记录页面
- 部署
  - Docker / Docker Compose
  - systemd 服务
  - 生产环境安装脚本
  - Agent 独立安装脚本
  - 备份与日志轮转
- 测试
  - Agent 单元测试
  - Server 单元测试
  - 真实进程端到端集成测试（SQLite + miniredis）
  - Playwright smoke 测试
- 文档
  - README（中英文）
  - 部署指南、API 文档、数据库 schema、贡献指南、行为准则

### 已知限制

- 自定义脚本指标未实现
- 离线检测任务未实现
- Token 轮换幂等缓存未实现
- `GetConfigResponse` / `RotateTokenResponse` 协议缺少 `message` 字段
- 当前环境未实际运行 Docker Compose 与 Playwright 浏览器测试

[Unreleased]: https://github.com/huang110/serverprobe/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/huang110/serverprobe/releases/tag/v1.0.0
