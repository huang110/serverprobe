/**
 * WebSocket 连接状态管理。
 * - 将三类推送分发到 Agent 缓存与全局通知
 * - 对高频 realtime_metrics 做批量节流（flush 间隔取用户刷新频率偏好）
 * - 维护「触发中告警」计数，供顶栏 KPI 展示
 */
import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import { wsClient } from '@/utils/websocket'
import { useAuthStore } from './auth'
import { useAgentStore } from './agent'
import { useToastStore } from './toast'
import { usePrefsStore } from './prefs'
import type {
  AgentStatusMessage,
  AlertMessage,
  RealtimeMetricsMessage
} from '@/types/metrics'

const SPARKLINE_LEN = 24

export const useWebSocketStore = defineStore('websocket', () => {
  const connected = ref(false)
  const firingAlertCount = ref(0)

  const state = {
    started: false,
    // 待合并的实时指标：agentId -> 最新一条
    pending: new Map<string, RealtimeMetricsMessage>(),
    flushTimer: 0 as ReturnType<typeof setInterval> | 0,
    // 偏好监听的取消函数
    prefsStop: null as null | (() => void)
  }

  /** 节流刷新间隔（秒），来自用户偏好，最小 1s */
  function flushIntervalMs(): number {
    const prefs = usePrefsStore()
    return Math.max(1, prefs.refreshInterval) * 1000
  }

  /** 批量应用积压的实时指标，避免高频消息引发频繁重渲染 */
  function flushMetrics() {
    if (state.pending.size === 0) return
    const agentStore = useAgentStore()
    state.pending.forEach((data) => {
      const patch: Record<string, unknown> = {
        cpuUsage: data.cpu_usage,
        memoryPercent: data.memory_percent,
        load1: data.load1,
        netRxBytes: data.net_rx_bytes,
        netTxBytes: data.net_tx_bytes,
        lastHeartbeatAt: data.timestamp,
        status: 'online'
      }
      // 可选字段：仅在消息中携带时才覆盖，避免 undefined 抹掉已有值
      if (data.cpu_steal !== undefined) patch.cpuSteal = data.cpu_steal
      if (data.swap_percent !== undefined) patch.swapPercent = data.swap_percent
      if (data.net_rx_total !== undefined) patch.netRxTotal = data.net_rx_total
      if (data.net_tx_total !== undefined) patch.netTxTotal = data.net_tx_total
      if (data.uptime !== undefined) patch.uptime = data.uptime
      if (data.latency !== undefined) patch.latencies = data.latency

      agentStore.patchMetrics(data.agent_id, patch)
      agentStore.pushSpark(data.agent_id, {
        cpu: data.cpu_usage,
        memory: data.memory_percent,
        load: data.load1,
        net: (data.net_rx_bytes + data.net_tx_bytes) / 2
      }, SPARKLINE_LEN)
    })
    state.pending.clear()
  }

  function startFlushLoop() {
    if (state.flushTimer) clearInterval(state.flushTimer)
    state.flushTimer = setInterval(flushMetrics, flushIntervalMs())
  }

  function connect(): void {
    const authStore = useAuthStore()
    if (state.started) return

    state.started = true

    wsClient.onStatusChange = (isConnected) => {
      connected.value = isConnected
    }

    // 实时指标：进入节流队列
    wsClient.on('realtime_metrics', (data: RealtimeMetricsMessage) => {
      state.pending.set(data.agent_id, data)
    })

    wsClient.on('agent_status', (data: AgentStatusMessage) => {
      const agentStore = useAgentStore()
      agentStore.updateStatus(data.agent_id, data.status, data.last_heartbeat_at)
      if (data.status === 'offline') firingAlertCount.value = Math.max(0, firingAlertCount.value)
    })

    wsClient.on('alert', (data: AlertMessage) => {
      const toastStore = useToastStore()
      const type = data.severity === 'critical' ? 'danger' : 'warning'
      toastStore.push(type, data.rule_name || '收到告警通知', data.message)
      if ((data.status || 'firing') === 'firing') firingAlertCount.value += 1
    })

    startFlushLoop()
    wsClient.connect(authStore.token)

    // 用户调整刷新频率偏好时，重启节流定时器
    const prefsStore = usePrefsStore()
    state.prefsStop = watch(
      () => prefsStore.refreshInterval,
      () => startFlushLoop()
    )
  }

  function disconnect(): void {
    wsClient.disconnect()
    state.started = false
    connected.value = false
    if (state.flushTimer) {
      clearInterval(state.flushTimer)
      state.flushTimer = 0
    }
    state.prefsStop?.()
    state.prefsStop = null
    state.pending.clear()
  }

  /**
   * 用当前登录态重新连接（登录/退出后升级或降级连接）。
   * 保留已注册的消息订阅与节流定时器，仅重开底层 socket。
   */
  function reconnectWithAuth(): void {
    const authStore = useAuthStore()
    wsClient.reconnect(authStore.token)
  }

  const connectionLabel = computed(() => (connected.value ? '实时' : '连接中'))

  return {
    connected,
    connectionLabel,
    firingAlertCount,
    connect,
    disconnect,
    reconnectWithAuth,
    flushMetrics
  }
})
