<script setup lang="ts">
/**
 * Agent 详情页：
 * - 顶部主机信息卡片
 * - 最新值指标卡（CPU/内存/负载/网络），含「相比1小时前」对比
 * - 时间范围选择（1h/6h/24h/7d + 自定义）
 * - CPU / 内存 / 负载 / 网络 四张历史折线图（超过 1000 点自动降采样）
 * - 磁盘、网卡实时列表（移动端堆叠卡片）
 * - WebSocket 更新最新值；标签编辑、删除 Agent
 */
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { EChartsOption } from 'echarts'
import MetricsChart from '@/components/MetricsChart.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import EmptyState from '@/components/EmptyState.vue'
import BaseModal from '@/components/BaseModal.vue'
import UsageBar from '@/components/UsageBar.vue'
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
import { useWebSocketStore } from '@/stores/websocket'

const route = useRoute()
const router = useRouter()
const toastStore = useToastStore()
const agentStore = useAgentStore()
const wsStore = useWebSocketStore()

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

// 1 小时前的快照（用于对比）
const hourAgo = reactive<Record<'cpu' | 'memory' | 'load', number | null>>({
  cpu: null,
  memory: null,
  load: null
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

/** 在历史点中找最接近目标时间戳的一个值 */
function valueNear(points: MetricPoint[], targetTs: number): number | null {
  if (!points.length) return null
  let best = points[0]
  let bestDiff = Math.abs(best.timestamp - targetTs)
  for (const p of points) {
    const diff = Math.abs(p.timestamp - targetTs)
    if (diff < bestDiff) {
      best = p
      bestDiff = diff
    }
  }
  // 容差 10 分钟
  return bestDiff <= 600 ? best.value : null
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

    // 计算 1 小时前对比值（仅当范围覆盖 1 小时前）
    const target = end - 3600
    if (start <= target) {
      hourAgo.cpu = valueNear(history.cpu, target)
      hourAgo.memory = valueNear(history.memory, target)
      hourAgo.load = valueNear(history.load, target)
    }
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

// ---------- 最新值 + 1 小时前对比 ----------
interface DeltaView {
  text: string
  positive: boolean // true=上升，false=下降
  show: boolean
}

function buildDelta(current: number | undefined, past: number | null): DeltaView {
  if (current === undefined || past === null) {
    return { text: '', positive: true, show: false }
  }
  const diff = current - past
  if (Math.abs(diff) < 0.05) return { text: '持平', positive: false, show: true }
  return {
    text: `${diff > 0 ? '+' : ''}${diff.toFixed(1)}`,
    positive: diff > 0,
    show: true
  }
}

const cpuDelta = computed(() => buildDelta(realtime.value?.cpuUsage, hourAgo.cpu))
const memDelta = computed(() => buildDelta(realtime.value?.memoryPercent, hourAgo.memory))
const loadDelta = computed(() => buildDelta(realtime.value?.load1, hourAgo.load))

const netTotal = computed(() =>
  realtime.value ? realtime.value.netRxBytes + realtime.value.netTxBytes : 0
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

/** 页面整体刷新（R 快捷键） */
async function reloadAll() {
  await Promise.all([loadRealtime(), loadHistory()])
}

onMounted(async () => {
  window.addEventListener('app:refresh', reloadAll)
  try {
    const detail = await getAgent(agentId)
    agent.value = detail.agent
    labels.value = detail.labels || {}
    await Promise.all([loadRealtime(), loadHistory()])
    wsStore.connect()

    // WebSocket 实时指标：更新最新值
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
  window.removeEventListener('app:refresh', reloadAll)
})

const labelEntries = computed(() => Object.entries(labels.value))
const hasRealtime = computed(() => !!realtime.value)
</script>

<template>
  <div>
    <!-- 返回 -->
    <button
      class="mb-4 inline-flex items-center gap-1.5 text-sm text-muted transition hover:text-content"
      @click="router.push('/agents')"
    >
      <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
        <path stroke-linecap="round" stroke-linejoin="round" d="M10.5 19.5L3 12m0 0l7.5-7.5M3 12h18" />
      </svg>
      返回列表
    </button>

    <!-- 加载骨架 -->
    <div v-if="loading" class="space-y-5" aria-busy="true" aria-label="加载 Agent 详情">
      <div class="card p-5">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div v-for="i in 9" :key="i">
            <div class="skeleton h-3 w-14"></div>
            <div class="skeleton mt-2 h-4 w-28"></div>
          </div>
        </div>
      </div>
      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <div v-for="i in 4" :key="i" class="card h-[280px] p-4">
          <div class="skeleton h-4 w-32"></div>
          <div class="skeleton mt-4 h-[220px] w-full"></div>
        </div>
      </div>
    </div>

    <template v-else-if="agent">
      <!-- 顶部信息卡片 -->
      <div class="card mb-5 p-5 sm:p-6">
        <div class="flex flex-wrap items-start justify-between gap-4">
          <div class="grid flex-1 grid-cols-1 gap-x-8 gap-y-4 sm:grid-cols-2 lg:grid-cols-3">
            <div>
              <p class="text-xs text-muted">主机名</p>
              <p class="mt-1 text-sm font-semibold text-content">{{ agent.hostname }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">操作系统</p>
              <p class="mt-1 text-sm text-content/90">{{ agent.os }} {{ agent.os_version }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">IP 地址</p>
              <p class="mt-1 font-mono text-sm text-content/90">{{ agent.ip || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">架构</p>
              <p class="mt-1 text-sm text-content/90">{{ agent.arch || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">CPU 核心</p>
              <p class="mt-1 text-sm text-content/90">{{ agent.cpu_cores }} 核</p>
            </div>
            <div>
              <p class="text-xs text-muted">总内存</p>
              <p class="mt-1 text-sm text-content/90">{{ formatBytes(agent.memory_total) }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">状态</p>
              <p class="mt-1"><StatusBadge :status="agent.status" /></p>
            </div>
            <div>
              <p class="text-xs text-muted">最后心跳</p>
              <p class="mt-1 text-sm text-content/90">{{ heartbeatText }}</p>
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

          <!-- 操作按钮：图标 + 文字 -->
          <div class="flex shrink-0 gap-2">
            <button class="btn-ghost px-3 py-1.5 text-xs" aria-label="编辑 Agent 标签" @click="openLabelModal">
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M16.862 4.487l1.687-1.688a1.875 1.875 0 112.652 2.652L10.582 16.07a4.5 4.5 0 01-1.897 1.13L6 18l.8-2.685a4.5 4.5 0 011.13-1.897l8.932-8.931zm0 0L19.5 7.125" />
              </svg>
              编辑标签
            </button>
            <button class="btn-danger px-3 py-1.5 text-xs" aria-label="删除该 Agent" @click="deleteConfirmVisible = true">
              <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M14.74 9l-.346 9m-4.788 0L9.26 9m9.968-3.21c.342.052.682.107 1.022.166m-1.022-.165L18.16 19.673a2.25 2.25 0 01-2.244 2.077H8.084a2.25 2.25 0 01-2.244-2.077L4.772 5.79m14.456 0a48.108 48.108 0 00-3.478-.397m-12 .562c.34-.059.68-.114 1.022-.165m0 0a48.11 48.11 0 013.478-.397m7.5 0v-.916c0-1.18-.91-2.164-2.09-2.201a51.964 51.964 0 00-3.32 0c-1.18.037-2.09 1.022-2.09 2.201v.916m7.5 0a48.667 48.667 0 00-7.5 0" />
              </svg>
              删除
            </button>
          </div>
        </div>
      </div>

      <!-- 最新值指标卡 + 1 小时对比 -->
      <div class="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
        <!-- CPU -->
        <div class="card p-4">
          <div class="flex items-center justify-between">
            <p class="text-xs text-muted">CPU 使用率</p>
            <span
              v-if="cpuDelta.show"
              class="rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
              :class="cpuDelta.text === '持平'
                ? 'bg-surface-2 text-muted'
                : cpuDelta.positive
                  ? 'bg-danger/15 text-danger'
                  : 'bg-success/15 text-success'"
              :title="'相比1小时前'"
            >{{ cpuDelta.text }}</span>
          </div>
          <p class="mt-1.5 font-mono text-xl font-bold text-content">
            {{ hasRealtime ? realtime!.cpuUsage.toFixed(1) + '%' : '--' }}
          </p>
          <UsageBar :value="realtime?.cpuUsage ?? 0" class="mt-2" />
        </div>

        <!-- 内存 -->
        <div class="card p-4">
          <div class="flex items-center justify-between">
            <p class="text-xs text-muted">内存使用率</p>
            <span
              v-if="memDelta.show"
              class="rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
              :class="memDelta.text === '持平'
                ? 'bg-surface-2 text-muted'
                : memDelta.positive
                  ? 'bg-danger/15 text-danger'
                  : 'bg-success/15 text-success'"
              :title="'相比1小时前'"
            >{{ memDelta.text }}</span>
          </div>
          <p class="mt-1.5 font-mono text-xl font-bold text-content">
            {{ hasRealtime ? realtime!.memoryPercent.toFixed(1) + '%' : '--' }}
          </p>
          <UsageBar :value="realtime?.memoryPercent ?? 0" class="mt-2" />
        </div>

        <!-- 负载 -->
        <div class="card p-4">
          <div class="flex items-center justify-between">
            <p class="text-xs text-muted">系统负载</p>
            <span
              v-if="loadDelta.show"
              class="rounded-full px-1.5 py-0.5 text-[10px] font-semibold"
              :class="loadDelta.text === '持平'
                ? 'bg-surface-2 text-muted'
                : loadDelta.positive
                  ? 'bg-warning/15 text-warning'
                  : 'bg-success/15 text-success'"
              :title="'相比1小时前'"
            >{{ loadDelta.text }}</span>
          </div>
          <p class="mt-1.5 font-mono text-xl font-bold text-content">
            {{ hasRealtime ? realtime!.load1.toFixed(2) : '--' }}
          </p>
          <p class="mt-2 text-[11px] text-muted">{{ agent.cpu_cores }} 核 · 比值
            <span :class="realtime && realtime.load1 / agent.cpu_cores > 1 ? 'text-warning font-semibold' : ''">
              {{ hasRealtime ? (realtime!.load1 / agent.cpu_cores).toFixed(2) : '--' }}
            </span>
          </p>
        </div>

        <!-- 网络 -->
        <div class="card p-4">
          <p class="text-xs text-muted">网络总速率</p>
          <p class="mt-1.5 font-mono text-xl font-bold text-content">
            {{ hasRealtime ? formatRate(netTotal) : '--' }}
          </p>
          <div class="mt-2 flex items-center justify-between text-[11px]">
            <span class="flex items-center gap-1 text-success">
              <svg class="h-3 w-3 animate-arrow-down" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M6 10 L1 4h3V1h4v3h3z" /></svg>
              {{ hasRealtime ? formatRate(realtime!.netRxBytes) : '--' }}
            </span>
            <span class="flex items-center gap-1 text-info">
              <svg class="h-3 w-3 animate-arrow-up" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true"><path d="M6 2 L1 8h3v3h4V8h3z" /></svg>
              {{ hasRealtime ? formatRate(realtime!.netTxBytes) : '--' }}
            </span>
          </div>
        </div>
      </div>

      <!-- 时间范围选择器 -->
      <div class="mb-5 flex flex-wrap items-center gap-2">
        <div class="flex flex-wrap rounded-lg border border-line bg-surface-2 p-0.5">
          <button
            v-for="opt in rangeOptions"
            :key="opt.key"
            class="rounded-md px-3 py-1.5 text-xs font-medium transition"
            :aria-pressed="activeRange === opt.key"
            :class="activeRange === opt.key ? 'bg-accent text-white' : 'text-muted hover:text-content'"
            @click="changeRange(opt.key)"
          >{{ opt.label }}</button>
          <button
            class="rounded-md px-3 py-1.5 text-xs font-medium transition"
            :aria-pressed="activeRange === 'custom'"
            :class="activeRange === 'custom' ? 'bg-accent text-white' : 'text-muted hover:text-content'"
            @click="activeRange = 'custom'"
          >自定义</button>
        </div>

        <div v-if="activeRange === 'custom'" class="flex flex-wrap items-center gap-2">
          <input v-model="customStart" type="datetime-local" aria-label="自定义开始时间" class="form-input w-auto py-1.5 text-xs" />
          <span class="text-muted">至</span>
          <input v-model="customEnd" type="datetime-local" aria-label="自定义结束时间" class="form-input w-auto py-1.5 text-xs" />
          <button class="btn-primary px-3 py-1.5 text-xs" @click="applyCustomRange">应用</button>
        </div>

        <span v-if="chartsLoading" class="ml-auto flex items-center gap-1.5 text-xs text-muted" aria-live="polite">
          <span class="h-3.5 w-3.5 animate-spin rounded-full border-2 border-line border-t-accent"></span>
          加载图表
        </span>
      </div>

      <!-- 图表区域（恰好 4 个 metrics-chart，勿改） -->
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
      <div class="card mb-6 overflow-hidden">
        <div class="border-b border-line px-5 py-3.5">
          <h3 class="text-sm font-semibold text-content">磁盘使用情况</h3>
        </div>
        <div class="overflow-x-auto">
          <table v-if="realtime?.disks.length" class="data-table stack">
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
                <span class="row-indicator"></span>
                <td data-label="挂载点" class="font-mono text-content/90">{{ disk.mount_point }}</td>
                <td data-label="使用率">
                  <div class="flex items-center gap-2">
                    <div class="h-1.5 w-24 overflow-hidden rounded-full bg-surface-3">
                      <div
                        class="h-full rounded-full transition-all duration-700"
                        :class="disk.usage_percent >= 85 ? 'bg-danger' : disk.usage_percent >= 70 ? 'bg-warning' : 'bg-success'"
                        :style="{ width: disk.usage_percent + '%' }"
                      ></div>
                    </div>
                    <span class="font-mono text-xs">{{ disk.usage_percent.toFixed(1) }}%</span>
                  </div>
                </td>
                <td data-label="读取速率" class="font-mono text-success">{{ formatRate(disk.read_bytes) }}</td>
                <td data-label="写入速率" class="font-mono text-info">{{ formatRate(disk.write_bytes) }}</td>
                <td data-label="IO 使用率" class="font-mono text-content/80">{{ disk.io_util.toFixed(1) }}%</td>
              </tr>
            </tbody>
          </table>
          <EmptyState v-else variant="chart" text="暂无磁盘数据" hint="实时上报中可能未包含磁盘明细" />
        </div>
      </div>

      <!-- 网卡实时列表 -->
      <div class="card overflow-hidden">
        <div class="border-b border-line px-5 py-3.5">
          <h3 class="text-sm font-semibold text-content">网络接口流量</h3>
        </div>
        <div class="overflow-x-auto">
          <table v-if="realtime?.networks.length" class="data-table stack">
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
                <span class="row-indicator"></span>
                <td data-label="接口" class="font-mono text-content/90">{{ net.interface }}</td>
                <td data-label="实时下行" class="font-mono text-success">{{ formatRate(net.rx_bytes) }}</td>
                <td data-label="实时上行" class="font-mono text-info">{{ formatRate(net.tx_bytes) }}</td>
                <td data-label="接收错误">
                  <span class="font-mono text-xs" :class="net.rx_errors > 0 ? 'text-warning' : 'text-muted'">{{ net.rx_errors }}</span>
                </td>
                <td data-label="发送错误">
                  <span class="font-mono text-xs" :class="net.tx_errors > 0 ? 'text-warning' : 'text-muted'">{{ net.tx_errors }}</span>
                </td>
              </tr>
            </tbody>
          </table>
          <EmptyState v-else variant="chart" text="暂无网络接口数据" hint="实时上报中可能未包含网卡明细" />
        </div>
      </div>

      <!-- 标签编辑弹窗 -->
      <BaseModal :show="labelModalVisible" title="编辑标签" @close="labelModalVisible = false">
        <div class="space-y-2.5">
          <div v-for="(item, index) in labelDraft" :key="index" class="flex items-center gap-2">
            <input v-model="item.key" type="text" aria-label="标签键" class="form-input" placeholder="键（如 env）" />
            <span class="text-muted">=</span>
            <input v-model="item.value" type="text" aria-label="标签值" class="form-input" placeholder="值（如 prod）" />
            <button
              class="shrink-0 rounded-md p-1.5 text-muted transition hover:bg-surface-3 hover:text-danger"
              aria-label="移除该标签"
              @click="removeLabelRow(index)"
            >
              <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <button class="btn-ghost w-full py-1.5 text-xs" aria-label="添加一行标签" @click="addLabelRow">+ 添加标签</button>
        </div>
        <template #footer>
          <button class="btn-ghost" @click="labelModalVisible = false">取消</button>
          <button class="btn-primary" @click="saveLabels">保存</button>
        </template>
      </BaseModal>

      <!-- 删除确认弹窗 -->
      <BaseModal :show="deleteConfirmVisible" title="删除确认" width="420px" @close="deleteConfirmVisible = false">
        <p class="text-sm leading-relaxed text-muted">
          确定要删除 Agent <span class="font-semibold text-content">{{ agent.hostname }}</span> 吗？
          此操作会同时清除其实时缓存，且不可恢复。
        </p>
        <template #footer>
          <button class="btn-ghost" @click="deleteConfirmVisible = false">取消</button>
          <button class="btn-danger" @click="confirmDelete">确认删除</button>
        </template>
      </BaseModal>
    </template>

    <EmptyState v-else variant="error" text="Agent 不存在或已被删除" hint="请返回列表查看当前在线的主机">
      <button class="btn-primary mt-2 text-xs" @click="router.push('/agents')">返回 Agent 列表</button>
    </EmptyState>
  </div>
</template>
