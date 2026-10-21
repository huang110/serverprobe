<script setup lang="ts">
/**
 * Agent 详情页：
 * - 顶部主机信息卡片
 * - 时间范围选择（1h/6h/24h/7d + 自定义）
 * - CPU / 内存 / 负载 / 网络 四张历史折线图
 * - 磁盘、网卡实时列表
 * - WebSocket 更新顶部信息与最新值
 * - 标签编辑、删除 Agent
 */
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { EChartsOption } from 'echarts'
import MetricsChart from '@/components/MetricsChart.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import EmptyState from '@/components/EmptyState.vue'
import BaseModal from '@/components/BaseModal.vue'
import { getAgent, updateAgentLabels, deleteAgent } from '@/api/agent'
import { getHistoryMetrics, getRealtimeMetrics } from '@/api/metrics'
import { buildLineOption } from '@/utils/charts'
import {
  formatBytes,
  formatRate,
  formatRelativeTime
} from '@/utils/format'
import type { Agent, AgentLabels } from '@/types/agent'
import type {
  HistoryMetric,
  MetricPoint,
  RealtimeMetrics
} from '@/types/metrics'
import { useAgentStore } from '@/stores/agent'
import { useToastStore } from '@/stores/toast'

const route = useRoute()
const router = useRouter()
const toastStore = useToastStore()
const agentStore = useAgentStore()

const agentId = route.params.id as string

const agent = ref<Agent | null>(null)
const labels = ref<AgentLabels>({})
const realtime = ref<RealtimeMetrics | null>(null)
const loading = ref(true)
const chartsLoading = ref(false)

// 各指标历史数据
const history = reactive<Record<HistoryMetric, MetricPoint[]>>({
  cpu: [],
  memory: [],
  load: [],
  network: []
})

// 时间范围
type RangeKey = '1h' | '6h' | '24h' | '7d' | 'custom'
const activeRange = ref<RangeKey>('1h')
const rangeOptions: { key: RangeKey; label: string; seconds: number }[] = [
  { key: '1h', label: '1小时', seconds: 3600 },
  { key: '6h', label: '6小时', seconds: 21600 },
  { key: '24h', label: '24小时', seconds: 86400 },
  { key: '7d', label: '7天', seconds: 604800 }
]
const customStart = ref('')
const customEnd = ref('')

// 每秒触发一次，用于刷新「最后心跳」等相对时间
const nowTick = ref(Date.now())
let tickTimer: ReturnType<typeof setInterval>
let realtimeTimer: ReturnType<typeof setInterval>
let unsubMetrics: (() => void) | null = null

function currentRange(): { start: number; end: number } {
  const end = Math.floor(Date.now() / 1000)
  if (activeRange.value === 'custom') {
    return {
      start: Math.floor(new Date(customStart.value).getTime() / 1000),
      end: Math.floor(new Date(customEnd.value).getTime() / 1000)
    }
  }
  const opt = rangeOptions.find((r) => r.key === activeRange.value)!
  return { start: end - opt.seconds, end }
}

/** 并行拉取四类历史指标 */
async function loadHistory() {
  if (activeRange.value === 'custom' && (!customStart.value || !customEnd.value)) return
  chartsLoading.value = true
  const { start, end } = currentRange()

  const metrics: HistoryMetric[] = ['cpu', 'memory', 'load', 'network']
  try {
    const results = await Promise.all(
      metrics.map((metric) => getHistoryMetrics(agentId, { start, end, metric }))
    )
    results.forEach((res, i) => {
      history[metrics[i]] = res.points || []
    })
  } catch (err) {
    toastStore.danger('加载历史指标失败', err instanceof Error ? err.message : '')
  } finally {
    chartsLoading.value = false
  }
}

/** 拉取实时指标（磁盘 / 网卡明细） */
async function loadRealtime() {
  try {
    realtime.value = await getRealtimeMetrics(agentId)
  } catch (err) {
    console.warn('实时指标暂不可用:', err)
  }
}

function changeRange(key: RangeKey) {
  activeRange.value = key
  if (key !== 'custom') loadHistory()
}

function applyCustomRange() {
  if (!customStart.value || !customEnd.value) {
    toastStore.warning('请选择完整的开始和结束时间')
    return
  }
  loadHistory()
}

// ---------- 图表配置 ----------
const cpuOption = computed<EChartsOption>(() =>
  buildLineOption('CPU 使用率', [{ name: 'CPU', color: '#22c55e', points: history.cpu }], '%', true)
)

const memoryOption = computed<EChartsOption>(() =>
  buildLineOption('内存使用率', [{ name: '内存', color: '#38bdf8', points: history.memory }], '%', true)
)

const loadOption = computed<EChartsOption>(() =>
  buildLineOption('系统负载（load1）', [{ name: 'load1', color: '#f5a524', points: history.load }])
)

const networkOption = computed<EChartsOption>(() =>
  buildLineOption(
    '网络接收速率',
    [{ name: '下行', color: '#3b82f6', points: history.network }]
  )
)

// ---------- 标签编辑 ----------
const labelModalVisible = ref(false)
const labelDraft = ref<{ key: string; value: string }[]>([])

function openLabelModal() {
  labelDraft.value = Object.entries(labels.value).map(([key, value]) => ({ key, value }))
  if (labelDraft.value.length === 0) labelDraft.value.push({ key: '', value: '' })
  labelModalVisible.value = true
}

function addLabelRow() {
  labelDraft.value.push({ key: '', value: '' })
}

function removeLabelRow(index: number) {
  labelDraft.value.splice(index, 1)
}

async function saveLabels() {
  const next: AgentLabels = {}
  for (const item of labelDraft.value) {
    const key = item.key.trim()
    if (key) next[key] = item.value
  }
  try {
    await updateAgentLabels(agentId, next)
    labels.value = next
    agentStore.setLabels(agentId, next)
    labelModalVisible.value = false
    toastStore.success('标签已更新')
  } catch (err) {
    toastStore.danger('更新标签失败', err instanceof Error ? err.message : '')
  }
}

// ---------- 删除 Agent ----------
const deleteConfirmVisible = ref(false)

async function confirmDelete() {
  try {
    await deleteAgent(agentId)
    agentStore.removeAgent(agentId)
    toastStore.success('Agent 已删除')
    router.push('/agents')
  } catch (err) {
    toastStore.danger('删除失败', err instanceof Error ? err.message : '')
  } finally {
    deleteConfirmVisible.value = false
  }
}

// 顶部展示用：优先使用实时数据
const heartbeatAt = computed(
  () => realtime.value?.lastHeartbeatAt || agent.value?.last_heartbeat_at || 0
)

// 依赖 nowTick，每秒重新计算相对时间
const heartbeatText = computed(() => {
  void nowTick.value
  return formatRelativeTime(heartbeatAt.value)
})

onMounted(async () => {
  try {
    const detail = await getAgent(agentId)
    agent.value = detail.agent
    labels.value = detail.labels || {}
    await Promise.all([loadRealtime(), loadHistory()])

    // WebSocket 实时指标：更新顶部最新值
    const { wsClient } = await import('@/utils/websocket')
    unsubMetrics = wsClient.on('realtime_metrics', (data) => {
      if (data.agent_id !== agentId) return
      if (!realtime.value) return
      realtime.value.cpuUsage = data.cpu_usage
      realtime.value.memoryPercent = data.memory_percent
      realtime.value.load1 = data.load1
      realtime.value.netRxBytes = data.net_rx_bytes
      realtime.value.netTxBytes = data.net_tx_bytes
      realtime.value.lastHeartbeatAt = data.timestamp
    })
  } catch (err) {
    toastStore.danger('加载 Agent 详情失败', err instanceof Error ? err.message : '')
  } finally {
    loading.value = false
  }

  tickTimer = setInterval(() => (nowTick.value = Date.now()), 1000)
  // 每 30 秒刷新一次实时磁盘/网卡明细
  realtimeTimer = setInterval(loadRealtime, 30000)
})

onBeforeUnmount(() => {
  clearInterval(tickTimer)
  clearInterval(realtimeTimer)
  unsubMetrics?.()
})

const labelEntries = computed(() => Object.entries(labels.value))
</script>

<template>
  <div>
    <!-- 返回 -->
    <button class="mb-4 inline-flex items-center gap-1 text-sm text-muted transition hover:text-white" @click="router.push('/agents')">
      <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
      </svg>
      返回列表
    </button>

    <LoadingSpinner v-if="loading" />

    <template v-else-if="agent">
      <!-- 顶部信息卡片 -->
      <div class="card mb-6 p-6">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="grid flex-1 grid-cols-1 gap-x-8 gap-y-4 sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <p class="text-xs text-muted">主机名</p>
              <p class="mt-1 text-sm font-semibold text-slate-100">{{ agent.hostname }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">操作系统</p>
              <p class="mt-1 text-sm text-slate-200">{{ agent.os }} {{ agent.os_version }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">IP 地址</p>
              <p class="mt-1 font-mono text-sm text-slate-200">{{ agent.ip || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">架构</p>
              <p class="mt-1 text-sm text-slate-200">{{ agent.arch || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">CPU 核心</p>
              <p class="mt-1 text-sm text-slate-200">{{ agent.cpu_cores }} 核</p>
            </div>
            <div>
              <p class="text-xs text-muted">总内存</p>
              <p class="mt-1 text-sm text-slate-200">{{ formatBytes(agent.memory_total) }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">状态</p>
              <p class="mt-1"><StatusBadge :status="agent.status" /></p>
            </div>
            <div>
              <p class="text-xs text-muted">最后心跳</p>
              <p class="mt-1 text-sm text-slate-200">{{ heartbeatText }}</p>
            </div>
            <div class="sm:col-span-2 lg:col-span-1">
              <p class="text-xs text-muted">标签</p>
              <div class="mt-1 flex flex-wrap gap-1.5">
                <template v-if="labelEntries.length">
                  <span
                    v-for="[key, value] in labelEntries"
                    :key="key"
                    class="rounded-md bg-accent/10 px-2 py-0.5 font-mono text-xs text-accent"
                  >{{ key }}={{ value }}</span>
                </template>
                <span v-else class="text-xs text-muted">无</span>
              </div>
            </div>
          </div>

          <!-- 操作按钮 -->
          <div class="flex shrink-0 gap-2">
            <button class="btn-ghost px-3 py-1.5 text-xs" @click="openLabelModal">编辑标签</button>
            <button class="btn-danger px-3 py-1.5 text-xs" @click="deleteConfirmVisible = true">删除</button>
          </div>
        </div>
      </div>

      <!-- 时间范围选择器 -->
      <div class="mb-5 flex flex-wrap items-center gap-2">
        <div class="flex flex-wrap rounded-lg border border-line bg-ink-700 p-0.5">
          <button
            v-for="opt in rangeOptions"
            :key="opt.key"
            class="rounded-md px-3 py-1.5 text-xs font-medium transition"
            :class="activeRange === opt.key ? 'bg-accent text-white' : 'text-muted hover:text-slate-200'"
            @click="changeRange(opt.key)"
          >{{ opt.label }}</button>
          <button
            class="rounded-md px-3 py-1.5 text-xs font-medium transition"
            :class="activeRange === 'custom' ? 'bg-accent text-white' : 'text-muted hover:text-slate-200'"
            @click="activeRange = 'custom'"
          >自定义</button>
        </div>

        <div v-if="activeRange === 'custom'" class="flex flex-wrap items-center gap-2">
          <input v-model="customStart" type="datetime-local" class="form-input w-auto py-1.5 text-xs" />
          <span class="text-muted">至</span>
          <input v-model="customEnd" type="datetime-local" class="form-input w-auto py-1.5 text-xs" />
          <button class="btn-primary px-3 py-1.5 text-xs" @click="applyCustomRange">应用</button>
        </div>

        <span v-if="chartsLoading" class="ml-auto flex items-center gap-1.5 text-xs text-muted">
          <span class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-line border-t-accent"></span>
          加载图表
        </span>
      </div>

      <!-- 图表区域 -->
      <div class="mb-6 grid grid-cols-1 gap-4 xl:grid-cols-2">
        <div class="card p-3">
          <MetricsChart :option="cpuOption" height="280px" :empty="!history.cpu.length" empty-text="该时间范围暂无 CPU 数据" />
        </div>
        <div class="card p-3">
          <MetricsChart :option="memoryOption" height="280px" :empty="!history.memory.length" empty-text="该时间范围暂无内存数据" />
        </div>
        <div class="card p-3">
          <MetricsChart :option="loadOption" height="280px" :empty="!history.load.length" empty-text="该时间范围暂无负载数据" />
        </div>
        <div class="card p-3">
          <MetricsChart :option="networkOption" height="280px" :empty="!history.network.length" empty-text="该时间范围暂无网络数据" />
        </div>
      </div>

      <!-- 磁盘实时列表 -->
      <div class="mb-6 card overflow-hidden">
        <div class="border-b border-line px-5 py-3.5">
          <h3 class="text-sm font-semibold text-slate-100">磁盘使用情况</h3>
        </div>
        <div class="overflow-x-auto">
          <table v-if="realtime?.disks.length" class="data-table">
            <thead>
              <tr>
                <th>挂载点</th>
                <th>使用率</th>
                <th>读取速率</th>
                <th>写入速率</th>
                <th>IO 使用率</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="disk in realtime.disks" :key="disk.mount_point">
                <td class="font-mono text-slate-200">{{ disk.mount_point }}</td>
                <td>
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-24 overflow-hidden rounded-full bg-ink-600">
                      <div class="h-full rounded-full" :class="disk.usage_percent >= 85 ? 'bg-danger' : disk.usage_percent >= 70 ? 'bg-warning' : 'bg-success'" :style="{ width: disk.usage_percent + '%' }"></div>
                    </div>
                    <span class="font-mono text-xs">{{ disk.usage_percent.toFixed(1) }}%</span>
                  </div>
                </td>
                <td class="font-mono text-success">{{ formatRate(disk.read_bytes) }}</td>
                <td class="font-mono text-info">{{ formatRate(disk.write_bytes) }}</td>
                <td class="font-mono">{{ disk.io_util.toFixed(1) }}%</td>
              </tr>
            </tbody>
          </table>
          <EmptyState v-else text="暂无磁盘数据" />
        </div>
      </div>

      <!-- 网卡实时列表 -->
      <div class="card overflow-hidden">
        <div class="border-b border-line px-5 py-3.5">
          <h3 class="text-sm font-semibold text-slate-100">网络接口流量</h3>
        </div>
        <div class="overflow-x-auto">
          <table v-if="realtime?.networks.length" class="data-table">
            <thead>
              <tr>
                <th>接口</th>
                <th>实时下行 ↓</th>
                <th>实时上行 ↑</th>
                <th>接收错误</th>
                <th>发送错误</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="net in realtime.networks" :key="net.interface">
                <td class="font-mono text-slate-200">{{ net.interface }}</td>
                <td class="font-mono text-success">{{ formatRate(net.rx_bytes) }}</td>
                <td class="font-mono text-info">{{ formatRate(net.tx_bytes) }}</td>
                <td>
                  <span :class="net.rx_errors > 0 ? 'text-warning' : 'text-muted'" class="font-mono text-xs">{{ net.rx_errors }}</span>
                </td>
                <td>
                  <span :class="net.tx_errors > 0 ? 'text-warning' : 'text-muted'" class="font-mono text-xs">{{ net.tx_errors }}</span>
                </td>
              </tr>
            </tbody>
          </table>
          <EmptyState v-else text="暂无网络接口数据" />
        </div>
      </div>

      <!-- 标签编辑弹窗 -->
      <BaseModal :show="labelModalVisible" title="编辑标签" @close="labelModalVisible = false">
        <div class="space-y-2.5">
          <div v-for="(item, index) in labelDraft" :key="index" class="flex items-center gap-2">
            <input v-model="item.key" type="text" class="form-input" placeholder="键（如 env）" />
            <span class="text-muted">=</span>
            <input v-model="item.value" type="text" class="form-input" placeholder="值（如 prod）" />
            <button class="shrink-0 rounded-md p-1.5 text-muted hover:bg-ink-600 hover:text-danger" @click="removeLabelRow(index)">
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <button class="btn-ghost w-full py-1.5 text-xs" @click="addLabelRow">+ 添加标签</button>
        </div>
        <template #footer>
          <button class="btn-ghost" @click="labelModalVisible = false">取消</button>
          <button class="btn-primary" @click="saveLabels">保存</button>
        </template>
      </BaseModal>

      <!-- 删除确认弹窗 -->
      <BaseModal :show="deleteConfirmVisible" title="删除确认" width="420px" @close="deleteConfirmVisible = false">
        <p class="text-sm text-slate-300">
          确定要删除 Agent <span class="font-semibold text-white">{{ agent.hostname }}</span> 吗？
          此操作会同时清除其实时缓存，且不可恢复。
        </p>
        <template #footer>
          <button class="btn-ghost" @click="deleteConfirmVisible = false">取消</button>
          <button class="btn-danger" @click="confirmDelete">确认删除</button>
        </template>
      </BaseModal>
    </template>

    <EmptyState v-else text="Agent 不存在或已被删除" />
  </div>
</template>
