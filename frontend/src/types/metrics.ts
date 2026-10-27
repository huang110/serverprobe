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

/** 实时 TCP Ping 目标延迟（realtime / WS 的 latency 数组元素） */
export interface LatencyLive {
  /** 目标名，如 "电信" / "联通" / "移动" / "阿里DNS" / "Cloudflare" */
  target_name: string
  /** 目标地址，如 "202.102.192.68:53" */
  target: string
  /** TCP Ping 延迟 (ms)，100% 丢包时为 0 */
  latency_ms: number
  /** 抖动 (ms) */
  jitter_ms: number
  /** 丢包率 (%)，0 ~ 100 */
  packet_loss: number
}

/** TCP Ping 历史中的单个数据点 */
export interface LatencyPoint {
  timestamp: number
  latency_ms: number
  jitter_ms: number
  packet_loss: number
}

/** 单个探测目标的 TCP Ping 历史序列 */
export interface LatencySeries {
  target_name: string
  target: string
  points: LatencyPoint[]
}

/** GET /metrics/latency 返回结构（data 字段） */
export interface LatencyHistoryResult {
  series: LatencySeries[]
}

/**
 * GET /api/agents/:id/metrics/realtime 返回结构（data 字段）。
 * disk / network 为以挂载点、网卡名为 key 的映射，在 API 层归一化为数组。
 * 单位约定：memory_used / swap_total / swap_used 为 MB；net_*_bytes / net_*_total 为 Bytes。
 */
export interface RealtimeMetricsRaw {
  cpu_usage: number
  /** 邻居偷跑 CPU % */
  cpu_steal?: number
  /** IO 等待 % */
  cpu_iowait?: number
  /** 已用内存（MB） */
  memory_used: number
  memory_percent: number
  /** Swap 总量（MB） */
  swap_total?: number
  /** Swap 已用（MB） */
  swap_used?: number
  swap_percent: number
  load1: number
  load5: number
  load15: number
  /** 实时下行速率 Bytes/s */
  net_rx_bytes: number
  /** 实时上行速率 Bytes/s */
  net_tx_bytes: number
  /** 累计下行总流量 Bytes */
  net_rx_total?: number
  /** 累计上行总流量 Bytes */
  net_tx_total?: number
  /** 系统持续运行秒数 */
  uptime?: number
  /** 进程数 */
  process_count?: number
  /** TCP Established 连接数 */
  tcp_established?: number
  /** TCP TimeWait 连接数 */
  tcp_time_wait?: number
  status: string
  last_heartbeat_at: number
  disk: Record<string, Partial<DiskLive>>
  network: Record<string, Partial<NetworkLive>>
  /** 多目标 TCP Ping 实时延迟 */
  latency?: LatencyLive[]
}

/** API 层归一化后的实时指标 */
export interface RealtimeMetrics {
  cpuUsage: number
  cpuSteal: number
  cpuIowait: number
  memoryUsed: number
  memoryPercent: number
  swapTotal: number
  swapUsed: number
  swapPercent: number
  load1: number
  load5: number
  load15: number
  netRxBytes: number
  netTxBytes: number
  netRxTotal: number
  netTxTotal: number
  uptime: number
  processCount: number
  tcpEstablished: number
  tcpTimeWait: number
  status: string
  lastHeartbeatAt: number
  disks: DiskLive[]
  networks: NetworkLive[]
  /** 多目标 TCP Ping 实时延迟 */
  latencies: LatencyLive[]
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
  cpu_steal?: number
  memory_percent: number
  swap_percent?: number
  load1: number
  net_rx_bytes: number
  net_tx_bytes: number
  net_rx_total?: number
  net_tx_total?: number
  uptime?: number
  /** 多目标 TCP Ping 实时延迟 */
  latency?: LatencyLive[]
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
