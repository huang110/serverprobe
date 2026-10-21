# ServerProbe 部署文档

## 前置条件

- Docker 20.10+ / Docker Compose 2.0+
- 或手动部署：Go 1.21+、MySQL 8.0、Redis 7、Node.js 18+（前端构建）

## 快速开始（Docker Compose）

```bash
cp .env.example .env
# 修改 .env 中的密码和密钥
./scripts/start.sh
```

访问 `http://localhost`，默认账号：

```text
用户名：admin
密码：admin123
```

服务端口：

| 服务 | 端口 |
|---|---|
| 前端 | 80 |
| HTTP API / WebSocket | 8080 |
| gRPC | 9090 |
| MySQL | 3306 |
| Redis | 6379 |

查看状态：

```bash
docker compose ps
docker compose logs -f server
```

停止：

```bash
./scripts/stop.sh
```

> 生产环境必须修改 `server/config.yaml` 中的 `agent.register_secret`、`jwt.secret`、`jwt.admin_password`，以及 MySQL/Redis 密码。

## 手动部署（systemd）

### 1. 初始化数据库

```bash
mysql -u root -p < docs/schema.sql
```

说明：Server 启动时会通过 GORM AutoMigrate 自动创建和补齐表结构，`docs/schema.sql` 主要用于手动初始化或 DBA 审核。

### 2. 安装 Server 和 Agent

```bash
sudo ./scripts/install-production.sh
```

安装目录：

- Server：`/opt/serverprobe/server`
- Agent：`/opt/serverprobe/agent`
- 日志：`/opt/serverprobe/server/logs`、`/var/log/serverprobe`

修改配置：

```bash
# install-production.sh 首次安装时会从 config.production.yaml 生成 config.yaml
sudo vim /opt/serverprobe/server/config.yaml
sudo vim /opt/serverprobe/agent/config.yaml
```

重启：

```bash
sudo systemctl restart serverprobe-server
sudo systemctl restart serverprobe-agent
```

### 3. 单独安装 Agent（被监控机器）

```bash
curl -fsSL https://your-domain.com/install-agent.sh | sudo bash -s -- \
  --server your-server:9090 \
  --key your-register-secret \
  --name "$(hostname)"
```

也可以在本机直接执行：

```bash
sudo ./scripts/install-agent.sh --server 1.2.3.4:9090 --key your-secret
```

### 4. 前端部署

构建：

```bash
cd frontend
npm ci
npm run build
```

将 `dist/` 部署到 Nginx，并参考 `frontend/nginx.conf` 配置 `/api/` 与 `/ws` 反向代理。

## 配置说明

配置优先级：环境变量 > `config.yaml` > 默认值。

关键环境变量：

| 环境变量 | 说明 |
|---|---|
| `SERVERPROBE_DATABASE_DSN` | MySQL DSN |
| `SERVERPROBE_REDIS_ADDR` | Redis 地址 |
| `SERVERPROBE_REDIS_PASSWORD` | Redis 密码 |
| `SERVERPROBE_JWT_SECRET` | JWT 密钥（至少 32 字符） |
| `SERVERPROBE_JWT_ADMIN_USERNAME` | 管理员用户名 |
| `SERVERPROBE_JWT_ADMIN_PASSWORD` | 管理员密码 |
| `SERVERPROBE_AGENT_REGISTER_SECRET` | Agent 注册密钥 |

完整配置参考 `server/config.production.yaml`。

## 健康检查

| 路径 | 说明 |
|---|---|
| `GET /api/health` | 进程存活 |
| `GET /api/health/ready` | MySQL / Redis 就绪 |
| `GET /api/health/metrics` | 简化 Prometheus 文本指标 |

Docker Compose 使用 `/api/health/ready` 作为健康检查。

## 备份

```bash
sudo DB_USER=serverprobe DB_PASS=your-password /opt/serverprobe/scripts/backup.sh
```

建议加入 crontab：

```cron
0 2 * * * DB_USER=serverprobe DB_PASS=your-password /opt/serverprobe/scripts/backup.sh >> /var/log/serverprobe-backup.log 2>&1
```

## 日志

- systemd：`journalctl -u serverprobe-server -f`
- Docker：`docker compose logs -f server`
- 日志轮转：参考 `scripts/logrotate.serverprobe`

## 测试

```bash
# 环境检查
./scripts/setup-test-env.sh

# 单元测试 + 端到端集成测试 + 前端构建
./scripts/run-integration-tests.sh

# 额外运行 Playwright UI 测试（需先启动 Server 和前端）
RUN_E2E=1 ./scripts/run-integration-tests.sh
```

集成测试会自动：

1. 编译 Server；
2. 启动本地 miniredis；
3. 使用临时 SQLite 数据库；
4. 启动真实的 Server HTTP/gRPC/WebSocket 进程；
5. 验证注册、心跳、实时数据、聚合、告警、WebSocket、并发注册。

## 故障排查

### Server 无法启动

```bash
systemctl status serverprobe-server
journalctl -u serverprobe-server -n 200 --no-pager
```

检查：

- `config.yaml` 是否存在
- MySQL / Redis 是否可连接
- 8080 / 9090 端口是否被占用
- `SERVERPROBE_JWT_SECRET` 是否为空

### Agent 无法注册

```bash
systemctl status serverprobe-agent
journalctl -u serverprobe-agent -n 200 --no-pager
```

检查：

- `server.address` 是否正确
- `register_secret` 是否与 Server 配置一致
- Server gRPC 9090 是否可访问
- TLS 模式是否匹配

### WebSocket 频繁断开

- 确认 Nginx 配置了 `proxy_read_timeout 86400`
- 确认 `/ws` 升级头正确
- 前端每 25 秒发送 ping，服务端 30 秒发送 pong

### 数据库数据过多

- 聚合任务会自动清理过期数据
- 原始指标保留 7 天，5m 保留 30 天，1h 保留 90 天，1d 保留 365 天
- 可手动调整 `internal/aggregator/cleaner.go` 中的保留天数

## 安全建议

1. 修改所有默认密码
2. JWT 密钥使用 32 位以上随机字符串
3. 生产环境启用 HTTPS / WSS
4. 不要将 MySQL、Redis、gRPC 端口直接暴露到公网
5. Server 以非 root 用户运行
6. 定期备份并验证恢复流程
7. 定期更新依赖和系统补丁
