/**
 * 指标相关类型定义。
 */

/** 实时磁盘指标（来自 GET /metrics/realtime 的 disk 映射） */
export interface DiskLive {
  mount_point: string
  /** 使用率 % */
  usage_percent: number
  /** 读取速率 Bytes/s */
  read_bytes: number
  /** 写入速率 Bytes/s */
  write_bytes: number
  /** IO 使用率 % */
  io_util: number
}

/** 实时网卡指标（来自 GET /metrics/realtime 的 network 映射） */
export interface NetworkLive {
  interface: string
  /** 入站速率 Bytes/s（后端 net_rx_bytes 实时值） */
  rx_bytes: number
  /** 出站速率 Bytes/s（后端 net_tx_bytes 实时值） */
  tx_bytes: number
  rx_errors: number
  tx_errors: number
}

/**
 * GET /api/agents/:id/metrics/realtime 返回结构（data 字段）。
 * disk / network 为以挂载点、网卡名为 key 的映射，在 API 层归一化为数组。
 */
export interface RealtimeMetricsRaw {
  cpu_usage: number
  memory_used: number
  memory_percent: number
  swap_percent: number
  load1: number
  load5: number
  load15: number
  net_rx_bytes: number
  net_tx_bytes: number
  status: string
  last_heartbeat_at: number
  disk: Record<string, Partial<DiskLive>>
  network: Record<string, Partial<NetworkLive>>
}

/** API 层归一化后的实时指标 */
export interface RealtimeMetrics {
  cpuUsage: number
  memoryUsed: number
  memoryPercent: number
  swapPercent: number
  load1: number
  load5: number
  load15: number
  netRxBytes: number
  netTxBytes: number
  status: string
  lastHeartbeatAt: number
  disks: DiskLive[]
  networks: NetworkLive[]
}

/** 历史指标中的单个数据点 */
export interface MetricPoint {
  timestamp: number
  value: number
}

/** GET /metrics/history 返回结构（data 字段） */
export interface HistoryResult {
  points: MetricPoint[]
  granularity: string
}

/** 历史指标可选类型，对应后端 metric 查询参数 */
export type HistoryMetric = 'cpu' | 'memory' | 'load' | 'network'

/** 聚合粒度 */
export type Granularity = '1m' | '5m' | '1h' | '1d'

/** WebSocket 推送的实时指标（精简版） */
export interface RealtimeMetricsMessage {
  agent_id: string
  cpu_usage: number
  memory_percent: number
  load1: number
  net_rx_bytes: number
  net_tx_bytes: number
  timestamp: number
}

/** WebSocket 推送的 Agent 状态变化 */
export interface AgentStatusMessage {
  agent_id: string
  status: string
  last_heartbeat_at: number
}

/** WebSocket 推送的告警通知 */
export interface AlertMessage {
  rule_id: number
  agent_id: string
  severity: string
  message: string
  fired_at: number
  /** 部分事件可能携带的可选字段 */
  rule_name?: string
  agent_hostname?: string
  status?: string
}

/** WebSocket 消息外层封装 */
export interface WsEnvelope<T = unknown> {
  type: 'realtime_metrics' | 'agent_status' | 'alert' | string
  data: T
}
