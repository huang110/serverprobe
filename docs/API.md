# ServerProbe HTTP API

统一成功响应：

```json
{
  "success": true,
  "data": {}
}
```

统一错误响应：

```json
{
  "success": false,
  "message": "error message"
}
```

## 认证与游客模式

### POST /api/auth/login

```json
{
  "username": "admin",
  "password": "your-password"
}
```

响应：

```json
{
  "success": true,
  "data": {
    "token": "jwt-token"
  }
}
```

### 公开只读接口

以下接口无需 JWT，可作为游客访问：

- `GET /api/agents`
- `GET /api/agents/:id`
- `GET /api/agents/:id/metrics/realtime`
- `GET /api/agents/:id/metrics/history`
- `GET /ws`（不传 token 时为游客只读连接）

携带有效 `Authorization: Bearer <JWT>` 时，返回完整数据。

游客访问 `GET /api/agents` / `GET /api/agents/:id` 时：

- `ip` 会脱敏，例如 `36.232.104.106` → `36.232.*.*`
- `internal_ip` 返回空字符串
- IPv6 保留前两段 hextet，其余掩码

### 管理接口（必须 JWT）

- `PUT /api/agents/:id`
- `DELETE /api/agents/:id`
- `GET /api/alert/rules`
- `POST /api/alert/rules`
- `PUT /api/alert/rules/:id`
- `DELETE /api/alert/rules/:id`
- `GET /api/alert/records`

## 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 进程存活 |
| GET | `/api/health/ready` | 检查 MySQL / Redis |
| GET | `/api/health/metrics` | 简化 Prometheus 文本指标 |

## Agent

### GET /api/agents

查询参数：

- `status`: `online` / `offline` / `all`
- `keyword`: 主机名或 IP
- `page`: 默认 1
- `page_size`: 默认 20，最大 100

### GET /api/agents/:id

返回 Agent 信息、Labels 和 Redis 实时指标。

### PUT /api/agents/:id

```json
{
  "labels": {
    "env": "prod",
    "owner": "devops"
  }
}
```

### DELETE /api/agents/:id

删除 Agent、Labels 和 Redis 实时缓存。

## 指标

### GET /api/agents/:id/metrics/realtime

响应 `data` 主要字段：

```json
{
  "cpu_usage": 25.5,
  "cpu_steal": 0.2,
  "cpu_iowait": 0.1,
  "memory_used": 2048,
  "memory_percent": 60.0,
  "swap_total": 1024,
  "swap_used": 128,
  "swap_percent": 12.5,
  "load1": 1.5,
  "net_rx_bytes": 1024,
  "net_tx_bytes": 512,
  "net_rx_total": 10737418240,
  "net_tx_total": 5368709120,
  "uptime": 518400,
  "process_count": 128,
  "tcp_established": 42,
  "tcp_time_wait": 5,
  "status": "online",
  "last_heartbeat_at": 1700000000,
  "disk": {},
  "network": {}
}
```

### GET /api/agents/:id/metrics/history

查询参数：

- `metric`: `cpu` / `memory` / `load` / `disk` / `network`
- `start`: Unix 秒
- `end`: Unix 秒
- `granularity`: `1m` / `5m` / `1h` / `1d`

未指定 granularity 时自动选择；磁盘只支持 `1m`。

## 告警

### POST /api/alert/rules

```json
{
  "name": "CPU High",
  "agent_id": "agent-1",
  "metric": "cpu_usage",
  "condition": "gt",
  "threshold": 80,
  "duration": 60,
  "severity": "critical",
  "webhook_url": "https://example.com/hook"
}
```

`agent_id` 为空表示全局规则。

### GET /api/alert/rules

查询参数：`agent_id`、`enabled`。

### PUT /api/alert/rules/:id

支持部分更新：

```json
{
  "name": "CPU Critical",
  "threshold": 90,
  "duration": 120,
  "severity": "critical",
  "webhook_url": "https://example.com/hook",
  "enabled": true
}
```

返回更新后的 `AlertRule` 对象。

### DELETE /api/alert/rules/:id

响应：

```json
{
  "success": true,
  "message": "rule deleted"
}
```

### GET /api/alert/records

查询参数：

- `agent_id`
- `severity`: info / warning / critical
- `status`: firing / resolved
- `start` / `end`: Unix 秒

## WebSocket

```
ws://host/ws?token=<JWT>
```

- 不传 `token` 时以游客身份连接，只读接收推送
- 传入 `token` 时验证 JWT；有效期为 JWT 过期时间
- 服务端每 10 秒推送在线 Agent 实时指标
- 每 15 秒巡检一次离线 Agent

客户端消息：

```json
{"type":"ping"}
```

服务端消息类型：

- `pong`
- `agent_status`
- `realtime_metrics`
- `alert`
