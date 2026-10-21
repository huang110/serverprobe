# ServerProbe HTTP API

除特别说明外，所有 `/api/agents`、`/api/alert`、`/api/metrics` 接口都需要：

```http
Authorization: Bearer <JWT>
```

统一响应格式：

```json
{
  "success": true,
  "data": {}
}
```

错误响应：

```json
{
  "success": false,
  "message": "error message"
}
```

## 认证

### POST /api/auth/login

```json
{
  "username": "admin",
  "password": "admin123"
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

## 健康检查

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/api/health` | 进程存活 |
| GET | `/api/health/ready` | 检查 MySQL、Redis |
| GET | `/api/health/metrics` | Prometheus 文本指标 |

## Agent

### GET /api/agents

查询参数：

- `status`：`online` / `offline` / `all`
- `keyword`：主机名或 IP
- `page`：默认 1
- `page_size`：默认 20，最大 100

### GET /api/agents/:id

返回 Agent 信息、Labels、Redis 实时指标。

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

返回 CPU、内存、负载、网络、磁盘和网络明细。

### GET /api/agents/:id/metrics/history

查询参数：

- `metric`：`cpu` / `memory` / `load` / `disk` / `network`
- `start`：Unix 秒
- `end`：Unix 秒
- `granularity`：`1m` / `5m` / `1h` / `1d`

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

### GET /api/alert/records

查询参数：

- `agent_id`
- `severity`：info / warning / critical
- `status`：firing / resolved
- `start` / `end`：Unix 秒

## WebSocket

```
ws://host/ws?token=<JWT>
```

客户端消息：

```json
{"type":"ping"}
```

服务端消息类型：

- `pong`
- `agent_status`
- `realtime_metrics`
- `alert`
