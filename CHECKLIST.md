# ServerProbe 交付检查清单

## 代码完整性

- [x] Agent 代码编译通过
- [x] Server 代码编译通过
- [x] Frontend 构建通过（TypeScript 类型检查）
- [x] Agent 单元测试通过
- [x] Server 单元测试通过
- [x] 端到端集成测试通过（注册、心跳、实时数据、聚合、告警、WebSocket、并发）
- [ ] 自定义脚本指标集成测试（当前 Agent 未实现该功能）

## 功能验证

- [x] Agent 成功注册到 Server
- [x] 心跳数据正常上报
- [x] 实时数据写入 Redis
- [x] 历史数据写入 MySQL / SQLite 分区
- [x] 5m 聚合任务正常执行
- [x] 1h / 1d 聚合链路单元测试通过
- [x] 告警规则可创建
- [x] 告警正确触发与恢复
- [x] WebSocket 可收到 realtime_metrics / alert 消息
- [x] 前端登录、Agent 列表、告警页面可访问
- [x] 前端 4 张 ECharts 图表在 Agent 详情页加载
- [ ] 前端 Playwright 测试在 CI 浏览器环境执行（测试代码已提供）

## 部署验证

- [x] Server Dockerfile 已提供
- [x] Agent Dockerfile 已提供
- [x] Frontend Dockerfile + nginx.conf 已提供
- [x] Docker Compose 配置已提供
- [x] Systemd 服务文件已提供
- [x] Agent 独立安装脚本已提供
- [x] 健康检查端点已实现
- [x] 日志输出正常
- [x] 备份脚本已提供
- [ ] 当前环境实际执行 `docker compose up`（环境无 Docker）

## 文档完整性

- [x] README.md 包含快速开始
- [x] docs/DEPLOYMENT.md 包含详细部署步骤
- [x] 配置文件有注释
- [x] 故障排查指南
- [x] 数据库 schema 提供
- [x] docs/API.md 包含 HTTP/WebSocket 接口说明

## 安全检查

- [x] 默认密码在文档中标注需修改
- [x] JWT 密钥支持环境变量
- [x] HMAC 签名验证正常
- [x] SQL 使用参数化查询
- [x] HTTP API 受 JWT 保护
- [x] CORS 配置
- [x] Token 黑名单使用 SHA256

## 性能验证

- [ ] 100 个 Agent 持续 5 分钟压力测试（集成测试提供 10 Agent 快速并发场景）
- [x] WebSocket 多连接支持（Hub 广播测试）
- [x] 聚合任务幂等
- [x] 前端生产构建成功

## 兼容性

- [x] Agent 在 Windows 开发环境测试通过
- [x] Agent 可交叉编译 Linux
- [x] 前端构建通过
- [ ] Safari / 移动端人工验收

## 已知问题

1. Agent 自定义脚本指标未实现，对应集成测试跳过。
2. 未实现独立离线检测任务，WebSocket 离线状态推送待补充。
3. Token 轮换幂等缓存未实现。
4. `GetConfigResponse` / `RotateTokenResponse` 协议无 `message` 字段。
5. Docker / Playwright 浏览器测试受当前执行环境限制，未实际运行。

## GitHub 发布准备

- [x] .gitignore
- [x] .gitattributes
- [x] LICENSE（MIT）
- [x] CONTRIBUTING.md
- [x] CODE_OF_CONDUCT.md
- [x] README.md / README_zh.md
- [x] CHANGELOG.md
- [x] GitHub Actions：CI / Docker / Release
- [x] Issue 模板 / PR 模板
- [x] 示例配置不含真实密钥
- [ ] 初始化 Git 仓库并推送到 GitHub（当前执行环境未安装 Git，需在本地执行）
- [ ] GitHub 仓库设置（分支保护、Topics、Secrets）

## 验证命令

```bash
# Agent
cd agent && go test -count=1 ./...

# Server
cd server && go test -count=1 ./...

# 集成测试
cd tests && go test -v -count=1 -timeout 10m ./...

# 前端
cd frontend && npm run build
```

## 签名

- 开发者：DeepSeek / OpenCode
- 审核者：待填写
- 交付日期：2026-10-21
