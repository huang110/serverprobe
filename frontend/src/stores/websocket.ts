/**
 * WebSocket 连接状态管理。
 * 在用户登录后建立连接，将三类推送消息分发到 Agent 数据缓存与全局通知。
 */
import { defineStore } from 'pinia'
import { ref } from 'vue'
import { wsClient } from '@/utils/websocket'
import { useAuthStore } from './auth'
import { useAgentStore } from './agent'
import { useToastStore } from './toast'
import type {
  AgentStatusMessage,
  AlertMessage,
  RealtimeMetricsMessage
} from '@/types/metrics'

export const useWebSocketStore = defineStore('websocket', () => {
  const connected = ref(false)
  const state = { started: false }

  /** 建立连接并注册消息处理（仅一次） */
  function connect(): void {
    const authStore = useAuthStore()
    if (!authStore.token || state.started) return

    state.started = true

    wsClient.onStatusChange = (isConnected) => {
      connected.value = isConnected
    }

    wsClient.on('realtime_metrics', (data: RealtimeMetricsMessage) => {
      const agentStore = useAgentStore()
      agentStore.patchMetrics(data.agent_id, {
        cpuUsage: data.cpu_usage,
        memoryPercent: data.memory_percent,
        load1: data.load1,
        netRxBytes: data.net_rx_bytes,
        netTxBytes: data.net_tx_bytes,
        lastHeartbeatAt: data.timestamp,
        status: 'online'
      })
    })

    wsClient.on('agent_status', (data: AgentStatusMessage) => {
      const agentStore = useAgentStore()
      agentStore.updateStatus(data.agent_id, data.status, data.last_heartbeat_at)
    })

    wsClient.on('alert', (data: AlertMessage) => {
      const toastStore = useToastStore()
      const type = data.severity === 'critical' ? 'danger' : 'warning'
      const title = data.rule_name || '收到告警通知'
      toastStore.push(type, title, data.message)
    })

    wsClient.connect(authStore.token)
  }

  /** 断开连接（退出登录时调用） */
  function disconnect(): void {
    wsClient.disconnect()
    state.started = false
    connected.value = false
  }

  return { connected, connect, disconnect }
})
