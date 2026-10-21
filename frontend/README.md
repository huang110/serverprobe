# ServerProbe 前端监控大屏

轻量级服务器监控系统 **ServerProbe** 的前端，基于 Vue 3 + TypeScript + Vite 构建，
提供 Agent 卡片大屏、主机详情图表、告警规则与记录管理，并通过 WebSocket 实时刷新。

## ✨ 功能特性

- **登录鉴权**：JWT Token 登录，自动携带请求头，401 自动跳转登录页
- **Agent 卡片大屏**：参考阿修罗监控风格，深色卡片网格，展示在线状态 / CPU / 内存 / 负载 / 上下行速率，数据更新时数值闪烁
- **Agent 详情**：主机信息、1h/6h/24h/7d 及自定义时间范围的 ECharts 历史图表、磁盘与网卡实时列表、标签编辑与删除
- **告警规则**：规则列表 + 创建弹窗（指标 / 条件 / 阈值 / 持续时间 / 严重级别 / Webhook）
- **告警记录**：Agent / 级别 / 状态 / 时间范围筛选，客户端分页，WebSocket 新告警实时插入
- **实时推送**：WebSocket 自动重连（5s）、25s 心跳、右上角全局通知
- **响应式**：移动端单列、桌面端多列网格，移动端独立导航栏

## 🧱 技术栈

| 类别 | 技术 |
| --- | --- |
| 框架 | Vue 3（`<script setup>`）+ TypeScript |
| 构建 | Vite 5 |
| 状态管理 | Pinia |
| 路由 | Vue Router 4（含登录守卫） |
| HTTP | Axios（请求 / 响应拦截器） |
| 图表 | Apache ECharts 5 |
| 样式 | TailwindCSS 3（深色主题） |
| 实时通信 | 原生 WebSocket 封装 |

## 📁 目录结构

```
frontend/
├── src/
│   ├── api/             # Axios 实例与各业务接口封装
│   │   ├── client.ts    #   统一 client（Token、401 处理）
│   │   ├── auth.ts
│   │   ├── agent.ts
│   │   ├── metrics.ts
│   │   └── alert.ts
│   ├── components/      # 通用组件（卡片、图表、弹窗、分页、徽章等）
│   ├── views/           # 页面：Login / AgentList / AgentDetail / AlertRules / AlertRecords
│   ├── router/          # 路由与守卫
│   ├── stores/          # Pinia：auth / agent / websocket / toast
│   ├── styles/          # Tailwind 入口与全局样式
│   ├── types/           # TypeScript 类型定义
│   ├── utils/           # 格式化、WebSocket、ECharts 配置工厂
│   ├── App.vue
│   └── main.ts
├── .env.development     # 开发环境后端地址
├── .env.production      # 生产环境后端地址（部署时修改）
├── tailwind.config.cjs
├── vite.config.ts
└── package.json
```

## 🚀 快速开始

### 前置条件

- Node.js ≥ 18
- 后端服务（ServerProbe Go Server）运行在 `http://localhost:8080`
- 默认账号：`admin` / `admin123`

### 开发模式

```bash
cd frontend
npm install
npm run dev
```

访问 <http://localhost:5173>。开发环境已配置代理，`/api` 与 `/ws` 请求会自动转发到
`http://localhost:8080`，无需关心跨域。

> Windows PowerShell 若禁止执行 `npm.ps1`，可使用 `npm.cmd`，或执行
> `Set-ExecutionPolicy -Scope CurrentUser RemoteSigned`。

### 生产构建

```bash
npm run build      # 先类型检查，再打包到 dist/
npm run preview    # 本地预览构建产物
```

## ⚙️ 环境变量

| 变量 | 说明 | 默认值 |
| --- | --- | --- |
| `VITE_API_BASE_URL` | HTTP API 基础地址；**留空**时走同源 / Vite 代理（`/api`） | 空 |
| `VITE_WS_BASE_URL` | WebSocket 基础地址（不含 `/ws`）；**留空**时走同源 / 代理（`/ws`） | 空 |

默认 `.env.development` 与 `.env.production` 均留空，由 Vite 开发代理或生产环境 Nginx
统一转发 `/api`、`/ws` 到后端，避免跨域。前后端分离部署时再填写实际域名，例如：

```
VITE_API_BASE_URL=https://probe.example.com
VITE_WS_BASE_URL=wss://probe.example.com
```

## 🧪 E2E 测试（Playwright）

测试文件位于 `e2e/`，运行前需启动后端（`http://localhost:8080`）并准备好 `admin/admin123`。

```bash
npm run test:e2e        # 无头运行（会自动拉起开发服务器）
npm run test:e2e:ui     # 交互式 UI 模式
```

也可指定已运行的前端地址：`E2E_BASE_URL=http://host:5173 npm run test:e2e`。

## 🐳 Docker 部署

仓库内置多阶段 `Dockerfile`（Node 构建 + Nginx 托管）与 `nginx.conf`
（SPA history 回退、`/api` 与 `/ws` 反向代理、静态资源缓存）：

```bash
docker build -t serverprobe-frontend .
docker run -p 80:80 serverprobe-frontend
```

Nginx 默认将后端代理到主机名 `server:8080`，可按实际网络调整 `nginx.conf`。

## 🔌 WebSocket

连接地址：`ws://<host>/ws?token=<JWT>`

| 消息类型 | 处理逻辑 |
| --- | --- |
| `realtime_metrics` | 更新对应 Agent 卡片 / 详情页最新指标（保留已加载的磁盘、网卡明细） |
| `agent_status` | 更新 Agent 在线状态与最后心跳时间 |
| `alert` | 右上角通知；在告警记录页插入到列表顶部 |

客户端每 25 秒发送 `{ "type": "ping" }`；断线后每 5 秒自动重连。

## ⚠️ 与任务书的接口差异说明（按真实后端实现）

开发前对后端代码进行了核对，实际接口与任务书存在以下差异，本项目均**以真实后端为准**：

1. **Agent 列表** `GET /api/agents`
   - 返回的是分页对象 `{ agents, total, page, page_size }`，而非数组；单页上限 100。
   - 前端通过 `listAllAgents()` 自动翻页拉取全部 Agent。
   - Agent 字段为 `memory_total`、`last_heartbeat_at`（秒），标签通过详情接口单独返回，
     列表对象内没有 `labels` 字段；`created_at`/`updated_at` 为 ISO 时间字符串。

2. **Agent 详情** `GET /api/agents/:id`
   - 返回 `{ agent, labels, realtime }`，实时指标需另调接口获取磁盘 / 网卡明细。

3. **实时指标** `GET /api/agents/:id/metrics/realtime`
   - 返回字段为 `cpu_usage / memory_used / memory_percent / load1 / load5 / load15 /
     net_rx_bytes / net_tx_bytes / disk / network`。
   - 其中 `disk`、`network` 是以挂载点、网卡名为 key 的**映射**，前端已归一化为数组。
   - Redis 实时缓存中磁盘仅含 `usage_percent / read_bytes / write_bytes / io_util`，
     **不含总量与已用容量**；网卡仅含实时速率与错误数，**不含累计总流量**。
     因此详情页磁盘表展示使用率与 IO、网卡表展示实时速率与错误计数（任务书中的总量列暂不可得）。

4. **历史指标** `GET /api/agents/:id/metrics/history`
   - 后端按 `metric=cpu|memory|load|network` **每次返回单一指标**，数据点为
     `{ timestamp, value }`，而非一次返回多列。前端并行发起 4 个请求。
   - `load` 历史仅记录 `load1`（无 load5/load15 的历史曲线）；
     `network` 历史仅记录 `net_rx_bytes`（下行速率），因此历史图中只画「网络接收速率」。
     load5/load15 与上行实时值仍会在实时数据中展示。

5. **告警规则**
   - 仅有 `GET /api/alert/rules` 与 `POST /api/alert/rules`，**没有更新 / 删除接口**。
     表格中的「编辑 / 删除」按钮会提示「当前后端未开放」，避免调用不存在的接口。
   - 创建接口在服务端会强制 `enabled=true`（请求体中的开关不生效）。

6. **告警记录** `GET /api/alert/records`
   - 返回的是**数组**（非 `{ total, records }` 分页结构），服务端按 `start/end`
     时间范围过滤，最多返回 100 条。前端在客户端做 20/50/100 分页。
   - 记录字段含 `metric / threshold / actual_value`，但没有 `rule_name`、
     `agent_hostname`；Agent 主机名由前端用 Agent 列表映射得到。

## 📝 说明

- 图表数据点超过 1500 时自动抽样，避免长时间范围下渲染卡顿。
- 所有 API 调用均有错误提示；列表与图表均有加载态与空状态。
- 图表、卡片为深色配色，数值更新使用 Tailwind 自定义 `flash` 动画高亮。
