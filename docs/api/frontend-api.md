# ServerProbe 前端 API 文档

> 本文档供前端开发使用，描述所有 HTTP API 和 WebSocket 接口

## 基础信息

- **Base URL**: `http://localhost:8080/api`
- **WebSocket URL**: `ws://localhost:8080/ws`
- **认证方式**: JWT Token（存储在 Cookie 或 Header）
- **响应格式**: JSON
- **时间格式**: Unix 时间戳（秒）

---

## 一、认证接口

### 1.1 登录

**POST** `/auth/login`

**请求体：**
```json
{
  "username": "admin",
  "password": "password"
}
```

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "token": "eyJhbGc...",
    "user": {
      "id": 1,
      "username": "admin",
      "role": "admin"
    }
  }
}
```

---

## 二、Agent 管理接口

### 2.1 获取 Agent 列表

**GET** `/agents`

**查询参数：**
- `page` (int): 页码，默认 1
- `page_size` (int): 每页数量，默认 20
- `status` (string): 状态筛选（online/offline/warning/error）
- `search` (string): 搜索关键词（主机名或 IP）
- `label` (string): 标签筛选（格式：key=value）

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 100,
    "page": 1,
    "page_size": 20,
    "items": [
      {
        "agent_id": "550e8400-e29b-41d4-a716-446655440000",
        "hostname": "web-server-01",
        "ip": "203.0.113.1",
        "internal_ip": "192.168.1.10",
        "os": "Linux",
        "os_version": "Ubuntu 22.04",
        "cpu_cores": 4,
        "memory_total": 8192,
        "disk_total": 102400,
        "status": "online",
        "last_heartbeat": 1729417800,
        "created_at": 1729317800,
        "labels": {
          "continent": "Asia",
          "isp": "AWS",
          "datacenter": "ap-southeast-1"
        },
        "metrics": {
          "cpu_usage": 45.6,
          "memory_usage": 67.8,
          "disk_usage": 42.3,
          "network_rx_speed": 1048576,
          "network_tx_speed": 524288
        }
      }
    ]
  }
}
```

### 2.2 获取单个 Agent 详情

**GET** `/agents/:agent_id`

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "hostname": "web-server-01",
    "ip": "203.0.113.1",
    "internal_ip": "192.168.1.10",
    "os": "Linux",
    "os_version": "Ubuntu 22.04",
    "kernel_version": "5.15.0-91-generic",
    "arch": "amd64",
    "cpu_cores": 4,
    "memory_total": 8192,
    "disk_total": 102400,
    "agent_version": "1.0.0",
    "status": "online",
    "last_heartbeat": 1729417800,
    "created_at": 1729317800,
    "updated_at": 1729417800,
    "labels": {
      "continent": "Asia",
      "isp": "AWS"
    }
  }
}
```

### 2.3 更新 Agent 标签

**PUT** `/agents/:agent_id/labels`

**请求体：**
```json
{
  "labels": {
    "continent": "Europe",
    "isp": "DigitalOcean"
  }
}
```

**响应：**
```json
{
  "code": 0,
  "message": "success"
}
```

### 2.4 删除 Agent

**DELETE** `/agents/:agent_id`

**响应：**
```json
{
  "code": 0,
  "message": "Agent deleted successfully"
}
```

---

## 三、监控数据接口

### 3.1 获取实时指标

**GET** `/agents/:agent_id/metrics/realtime`

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "timestamp": 1729417800,
    "cpu": {
      "usage_percent": 45.6,
      "user_percent": 30.2,
      "system_percent": 15.4,
      "idle_percent": 54.4,
      "iowait_percent": 0.8,
      "per_core_usage": [42.1, 48.3, 46.7, 45.2]
    },
    "memory": {
      "total": 8192,
      "used": 5554,
      "free": 2638,
      "available": 3210,
      "usage_percent": 67.8,
      "swap_total": 4096,
      "swap_used": 512,
      "swap_usage_percent": 12.5
    },
    "disks": [
      {
        "device": "/dev/sda1",
        "mount_point": "/",
        "fs_type": "ext4",
        "total": 102400,
        "used": 43315,
        "free": 59085,
        "usage_percent": 42.3,
        "inode_usage_percent": 5.2,
        "read_bytes_per_sec": 1048576,
        "write_bytes_per_sec": 524288,
        "io_util_percent": 15.6
      }
    ],
    "networks": [
      {
        "interface": "eth0",
        "ip_address": "192.168.1.10",
        "rx_bytes_per_sec": 1048576,
        "tx_bytes_per_sec": 524288,
        "rx_packets": 1250,
        "tx_packets": 890
      }
    ],
    "system": {
      "load1": 2.5,
      "load5": 2.3,
      "load15": 2.1,
      "uptime": 1234567,
      "process_count": 156,
      "tcp_established": 45,
      "tcp_time_wait": 12
    }
  }
}
```

### 3.2 获取历史数据

**GET** `/agents/:agent_id/metrics/history`

**查询参数：**
- `metric` (string): 指标名称（cpu.usage/memory.usage/disk.usage）
- `start_time` (int): 开始时间（Unix 秒）
- `end_time` (int): 结束时间（Unix 秒）
- `granularity` (string): 粒度（1m/5m/1h/1d/7d），默认 1m

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "metric": "cpu.usage_percent",
    "granularity": "5m",
    "points": [
      {
        "timestamp": 1729414200,
        "avg_value": 45.6,
        "max_value": 67.8,
        "min_value": 23.4
      },
      {
        "timestamp": 1729414500,
        "avg_value": 48.2,
        "max_value": 70.1,
        "min_value": 25.6
      }
    ]
  }
}
```

### 3.3 获取 IP 质量信息

**GET** `/agents/:agent_id/ip-quality`

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "public_ip": "203.0.113.1",
    "isp": "Amazon Web Services",
    "asn": "AS16509",
    "country": "Singapore",
    "region": "Central Singapore",
    "city": "Singapore",
    "latitude": 1.3521,
    "longitude": 103.8198,
    "latency_tests": [
      {
        "target": "8.8.8.8",
        "target_name": "Google DNS",
        "latency_ms": 15.2,
        "jitter_ms": 2.3,
        "packet_loss": 0,
        "ttl": 54
      }
    ],
    "quality_score": {
      "score": 95,
      "is_proxy": false,
      "is_vpn": false,
      "is_datacenter": true,
      "risk_level": "low",
      "isp_type": "hosting"
    },
    "last_check_time": 1729417800
  }
}
```

---

## 四、告警接口

### 4.1 获取告警列表

**GET** `/alerts`

**查询参数：**
- `page` (int): 页码
- `page_size` (int): 每页数量
- `status` (string): 状态（firing/resolved）
- `severity` (string): 严重性（info/warning/critical）
- `agent_id` (string): Agent ID

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "total": 50,
    "items": [
      {
        "alert_id": "alert-123",
        "agent_id": "550e8400-e29b-41d4-a716-446655440000",
        "hostname": "web-server-01",
        "rule_id": "rule-456",
        "metric_name": "cpu.usage_percent",
        "current_value": 95.6,
        "threshold": 80,
        "severity": "critical",
        "message": "CPU 使用率超过 80%（当前 95.6%）",
        "status": "firing",
        "fired_at": 1729417800,
        "resolved_at": null,
        "notified": true
      }
    ]
  }
}
```

### 4.2 获取告警规则

**GET** `/alert-rules`

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": [
    {
      "rule_id": "rule-456",
      "agent_id": null,
      "metric_name": "cpu.usage_percent",
      "rule_type": "consecutive",
      "threshold": 80,
      "operator": ">",
      "consecutive_count": 3,
      "severity": "warning",
      "silence_duration": 600,
      "notify_on_recovery": true,
      "enabled": true
    }
  ]
}
```

### 4.3 创建告警规则

**POST** `/alert-rules`

**请求体：**
```json
{
  "agent_id": null,
  "metric_name": "memory.usage_percent",
  "rule_type": "threshold",
  "threshold": 90,
  "operator": ">",
  "severity": "critical",
  "silence_duration": 600,
  "notify_on_recovery": true,
  "notification_channels": ["channel-1", "channel-2"]
}
```

---

## 五、WebSocket 实时推送

### 5.1 连接 WebSocket

**URL**: `ws://localhost:8080/ws?token=<jwt_token>`

### 5.2 消息格式

#### 客户端发送：订阅 Agent

```json
{
  "type": "subscribe",
  "data": {
    "agent_ids": ["agent-1", "agent-2"]
  }
}
```

#### 服务端推送：实时指标更新

```json
{
  "type": "metrics_update",
  "timestamp": 1729417800,
  "data": {
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "hostname": "web-server-01",
    "status": "online",
    "metrics": {
      "cpu": {"usage": 45.6},
      "memory": {"usage_percent": 67.8},
      "network": {
        "rx_speed": 1048576,
        "tx_speed": 524288
      }
    }
  }
}
```

#### 服务端推送：告警通知

```json
{
  "type": "alert",
  "timestamp": 1729417800,
  "data": {
    "alert_id": "alert-123",
    "agent_id": "550e8400-e29b-41d4-a716-446655440000",
    "hostname": "web-server-01",
    "severity": "critical",
    "message": "CPU 使用率超过 80%"
  }
}
```

---

## 六、系统配置接口

### 6.1 获取系统配置

**GET** `/system/config`

**响应：**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "register_secret": "***",
    "default_heartbeat_interval": 30,
    "data_retention_days": 90,
    "enable_ip_quality": false
  }
}
```

### 6.2 导出配置

**GET** `/system/export`

**响应：** JSON 文件下载

### 6.3 导入配置

**POST** `/system/import`

**请求体：** multipart/form-data（上传 JSON 文件）

---

## 七、错误码

| Code | Message | 说明 |
|------|---------|------|
| 0 | success | 成功 |
| 400 | bad request | 请求参数错误 |
| 401 | unauthorized | 未授权 |
| 403 | forbidden | 权限不足 |
| 404 | not found | 资源不存在 |
| 500 | internal server error | 服务器错误 |

---

## 八、前端开发建议

### 8.1 技术栈
- Vue 3 + Vite
- TailwindCSS（样式）
- ECharts（图表）
- Pinia（状态管理）
- Axios（HTTP 请求）
- Socket.io-client（WebSocket）

### 8.2 页面结构
```
/                   # 首页（Agent 列表卡片）
/agents/:id         # Agent 详情页（实时指标 + 折线图）
/agents/:id/history # 历史数据查询
/alerts             # 告警列表
/rules              # 告警规则管理
/settings           # 系统设置
```

### 8.3 响应式设计
- 桌面端：卡片 4 列布局
- 平板端：卡片 2 列布局
- 移动端：卡片 1 列布局

### 8.4 暗黑模式
使用 CSS 变量实现主题切换

### 8.5 国际化
使用 vue-i18n，支持中英文切换

---

## 联系方式

如有问题请联系后端开发团队。
