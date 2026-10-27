/**
 * Agent 数据状态管理。
 * 维护 Agent 列表、标签，以及各 Agent 的实时指标（卡片 + 详情页共用）。
 */
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import * as agentApi from '@/api/agent'
import { getRealtimeMetrics } from '@/api/metrics'
import type { Agent } from '@/types/agent'
import type { RealtimeMetrics } from '@/types/metrics'
import type { AgentLabels } from '@/types/agent'

export interface SparkPoint {
  cpu: number
  memory: number
  load: number
  net: number
}

export const useAgentStore = defineStore('agent', () => {
  const agents = ref<Agent[]>([])
  const labelsMap = ref<Record<string, AgentLabels>>({})
  const metricsMap = ref<Record<string, RealtimeMetrics>>({})
  const sparkMap = ref<Record<string, SparkPoint[]>>({})
  const loading = ref(false)
  const loaded = ref(false)

  const total = computed(() => agents.value.length)
  const onlineCount = computed(
    () => agents.value.filter((a) => a.status === 'online').length
  )

  /** 拉取全部 Agent 列表 */
  async function fetchAgents(keyword = ''): Promise<void> {
    loading.value = true
    try {
      agents.value = await agentApi.listAllAgents(keyword)
      loaded.value = true
    } finally {
      loading.value = false
    }
  }

  /** 拉取单个 Agent 的实时指标并缓存 */
  async function fetchRealtime(agentId: string): Promise<RealtimeMetrics | null> {
    try {
      const metrics = await getRealtimeMetrics(agentId)
      metricsMap.value[agentId] = metrics
      return metrics
    } catch (err) {
      // 离线 Agent 可能没有实时指标
      console.warn(`[agent] 获取 ${agentId} 实时指标失败:`, err)
      return null
    }
  }

  /** 为所有在线 Agent 批量加载实时指标（列表页初次渲染） */
  async function fetchAllRealtime(): Promise<void> {
    const onlineIds = agents.value
      .filter((a) => a.status === 'online')
      .map((a) => a.id)
    await Promise.all(onlineIds.map((id) => fetchRealtime(id)))
  }

  /** 获取单个 Agent */
  function getAgentById(agentId: string): Agent | undefined {
    return agents.value.find((a) => a.id === agentId)
  }

  /** 更新 Agent 状态（WebSocket 推送） */
  function updateStatus(agentId: string, status: string, heartbeatAt?: number): void {
    const agent = getAgentById(agentId)
    if (!agent) return
    agent.status = status
    if (heartbeatAt) agent.last_heartbeat_at = heartbeatAt
  }

  /**
   * 用 WebSocket 推送的精简实时指标合并到缓存。
   * 保留已有的 disk / network 明细（WS 消息中不含）。
   */
  function patchMetrics(
    agentId: string,
    patch: Partial<RealtimeMetrics>
  ): void {
    const existing = metricsMap.value[agentId]
    if (existing) {
      Object.assign(existing, patch)
    } else {
      metricsMap.value[agentId] = {
        cpuUsage: 0,
        cpuSteal: 0,
        cpuIowait: 0,
        memoryUsed: 0,
        memoryPercent: 0,
        swapTotal: 0,
        swapUsed: 0,
        swapPercent: 0,
        load1: 0,
        load5: 0,
        load15: 0,
        netRxBytes: 0,
        netTxBytes: 0,
        netRxTotal: 0,
        netTxTotal: 0,
        uptime: 0,
        processCount: 0,
        tcpEstablished: 0,
        tcpTimeWait: 0,
        status: 'online',
        lastHeartbeatAt: 0,
        disks: [],
        networks: [],
        ...patch
      }
    }
  }

  /** 删除本地缓存的 Agent（删除接口调用成功后） */
  function removeAgent(agentId: string): void {
    agents.value = agents.value.filter((a) => a.id !== agentId)
    delete metricsMap.value[agentId]
    delete labelsMap.value[agentId]
  }

  /** 保存标签到本地缓存 */
  function setLabels(agentId: string, labels: AgentLabels): void {
    labelsMap.value[agentId] = labels
  }

  /**
   * 批量拉取所有 Agent 的标签（GET /api/agents/:id 返回 labels）。
   * 标签只在详情接口中提供，列表页为展示 MJJ 地区/线路/价格标签需逐个拉取。
   * 控制并发数，避免一次性发出大量请求；失败静默跳过。
   */
  async function fetchAllLabels(concurrency = 6): Promise<void> {
    const queue = agents.value.map((a) => a.id)
    async function worker() {
      while (queue.length) {
        const id = queue.shift()!
        try {
          const detail = await agentApi.getAgent(id)
          if (detail.labels) labelsMap.value[id] = detail.labels
        } catch {
          // 单个失败不影响整体
        }
      }
    }
    await Promise.all(
      Array.from({ length: Math.min(concurrency, queue.length) }, () => worker())
    )
  }

  /** 追加一个 sparkline 趋势点（环形缓冲，固定长度） */
  function pushSpark(agentId: string, point: SparkPoint, maxLen: number): void {
    const list = sparkMap.value[agentId]
    if (!list) {
      sparkMap.value[agentId] = [point]
      return
    }
    list.push(point)
    if (list.length > maxLen) list.splice(0, list.length - maxLen)
  }

  return {
    agents,
    labelsMap,
    metricsMap,
    sparkMap,
    loading,
    loaded,
    total,
    onlineCount,
    fetchAgents,
    fetchRealtime,
    fetchAllRealtime,
    getAgentById,
    updateStatus,
    patchMetrics,
    removeAgent,
    setLabels,
    fetchAllLabels,
    pushSpark
  }
})
