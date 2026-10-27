/** 指标查询接口 */
import apiClient from './client'
import type {
  HistoryMetric,
  HistoryResult,
  RealtimeMetrics,
  RealtimeMetricsRaw
} from '@/types/metrics'

export interface HistoryParams {
  start: number
  end: number
  metric: HistoryMetric
  granularity?: string
}

/**
 * GET /api/agents/:id/metrics/realtime
 * 将后端返回的 disk / network 映射归一化为数组。
 */
export async function getRealtimeMetrics(agentId: string): Promise<RealtimeMetrics> {
  const { data } = await apiClient.get(`/api/agents/${agentId}/metrics/realtime`)
  if (!data?.success) throw new Error(data?.message || '获取实时指标失败')

  const raw = data.data as RealtimeMetricsRaw
  return normalizeRealtime(raw)
}

/** GET /api/agents/:id/metrics/history 查询历史指标 */
export async function getHistoryMetrics(
  agentId: string,
  params: HistoryParams
): Promise<HistoryResult> {
  const { data } = await apiClient.get(`/api/agents/${agentId}/metrics/history`, { params })
  if (!data?.success) throw new Error(data?.message || '获取历史指标失败')
  return data.data as HistoryResult
}

/** 归一化实时指标：将 keyed map 转为数组 */
export function normalizeRealtime(raw: RealtimeMetricsRaw): RealtimeMetrics {
  const disks = Object.entries(raw.disk || {}).map(([mountPoint, value]) => ({
    mount_point: value.mount_point ?? mountPoint,
    usage_percent: Number(value.usage_percent ?? 0),
    read_bytes: Number(value.read_bytes ?? 0),
    write_bytes: Number(value.write_bytes ?? 0),
    io_util: Number(value.io_util ?? 0)
  }))

  const networks = Object.entries(raw.network || {}).map(([iface, value]) => ({
    interface: value.interface ?? iface,
    rx_bytes: Number(value.rx_bytes ?? 0),
    tx_bytes: Number(value.tx_bytes ?? 0),
    rx_errors: Number(value.rx_errors ?? 0),
    tx_errors: Number(value.tx_errors ?? 0)
  }))

  return {
    cpuUsage: Number(raw.cpu_usage ?? 0),
    cpuSteal: Number(raw.cpu_steal ?? 0),
    cpuIowait: Number(raw.cpu_iowait ?? 0),
    memoryUsed: Number(raw.memory_used ?? 0),
    memoryPercent: Number(raw.memory_percent ?? 0),
    swapTotal: Number(raw.swap_total ?? 0),
    swapUsed: Number(raw.swap_used ?? 0),
    swapPercent: Number(raw.swap_percent ?? 0),
    load1: Number(raw.load1 ?? 0),
    load5: Number(raw.load5 ?? 0),
    load15: Number(raw.load15 ?? 0),
    netRxBytes: Number(raw.net_rx_bytes ?? 0),
    netTxBytes: Number(raw.net_tx_bytes ?? 0),
    netRxTotal: Number(raw.net_rx_total ?? 0),
    netTxTotal: Number(raw.net_tx_total ?? 0),
    uptime: Number(raw.uptime ?? 0),
    processCount: Number(raw.process_count ?? 0),
    tcpEstablished: Number(raw.tcp_established ?? 0),
    tcpTimeWait: Number(raw.tcp_time_wait ?? 0),
    status: raw.status ?? '',
    lastHeartbeatAt: Number(raw.last_heartbeat_at ?? 0),
    disks,
    networks
  }
}
