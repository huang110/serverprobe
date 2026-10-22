<script setup lang="ts">
/**
 * 告警记录页。
 * - 顶部统计卡：今日告警、触发中、严重、平均持续时长
 * - Agent/级别/状态/时间范围筛选（分段滑动）
 * - 表头点击排序
 * - 客户端分页，WS 新告警插入顶部
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import EmptyState from '@/components/EmptyState.vue'
import SeverityBadge from '@/components/SeverityBadge.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import StatCard from '@/components/StatCard.vue'
import SortableTh from '@/components/SortableTh.vue'
import { listRecords } from '@/api/alert'
import { listAllAgents } from '@/api/agent'
import { wsClient } from '@/utils/websocket'
import { formatDateTime, formatDuration, formatRelativeTime } from '@/utils/format'
import { useToastStore } from '@/stores/toast'
import type { AlertRecord } from '@/types/alert'
import type { Agent } from '@/types/agent'
import type { AlertMessage } from '@/types/metrics'

const toastStore = useToastStore()

const records = ref<AlertRecord[]>([])
const agents = ref<Agent[]>([])
const loading = ref(false)

const agentFilter = ref('')
const severityFilter = ref('')
const statusFilter = ref('')
const rangeKey = ref<'24h' | '7d' | '30d'>('24h')

const page = ref(1)
const pageSize = ref(20)

// 排序
type SortDir = 'asc' | 'desc' | null
const sortField = ref<string>('fired_at')
const sortDir = ref<SortDir>('desc')

function onSort(payload: { field: string; dir: SortDir }) {
  sortField.value = payload.field
  sortDir.value = payload.dir
}

// 筛选条件变化：回到第一页并重新加载
watch([agentFilter, severityFilter, statusFilter, rangeKey], () => {
  page.value = 1
  loadRecords()
})

async function loadRecords() {
  loading.value = true
  try {
    const secondsMap = { '24h': 86400, '7d': 604800, '30d': 2592000 }
    const end = Math.floor(Date.now() / 1000)
    const start = end - secondsMap[rangeKey.value]

    records.value = await listRecords({
      start,
      end,
      agent_id: agentFilter.value || undefined,
      severity: severityFilter.value || undefined,
      status: statusFilter.value || undefined
    })
  } catch (err) {
    toastStore.danger('加载告警记录失败', err instanceof Error ? err.message : '')
  } finally {
    loading.value = false
  }
}

function agentHostname(id: string): string {
  return agents.value.find((a) => a.id === id)?.hostname || id.slice(0, 8)
}

// ---------- 统计卡 ----------
const startOfToday = () => {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return Math.floor(d.getTime() / 1000)
}

const todayRecords = computed(() => records.value.filter((r) => r.fired_at >= startOfToday()))
const firingRecords = computed(() => records.value.filter((r) => r.status === 'firing'))
const criticalRecords = computed(() => records.value.filter((r) => r.severity === 'critical'))
const avgDuration = computed(() => {
  const resolved = records.value.filter((r) => r.resolved_at > 0)
  if (!resolved.length) return '—'
  const total = resolved.reduce((sum, r) => sum + (r.resolved_at - r.fired_at), 0)
  return formatDuration(0, Math.round(total / resolved.length))
})

// 筛选 + 排序后的列表
const sortedRecords = computed(() => {
  let list = [...records.value]
  if (sortDir.value && sortField.value) {
    const field = sortField.value as keyof AlertRecord
    list.sort((a, b) => {
      const av = a[field]
      const bv = b[field]
      if (av === bv) return 0
      const cmp = av > bv ? 1 : -1
      return sortDir.value === 'asc' ? cmp : -cmp
    })
  }
  return list
})

const pagedRecords = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return sortedRecords.value.slice(start, start + pageSize.value)
})

let unsubAlert: (() => void) | null = null
onMounted(async () => {
  try {
    agents.value = await listAllAgents()
  } catch {
    /* ignore */
  }
  await loadRecords()
  window.addEventListener('app:refresh', loadRecords)

  unsubAlert = wsClient.on('alert', (data: AlertMessage) => {
    const synthetic: AlertRecord = {
      id: Date.now(),
      rule_id: data.rule_id,
      agent_id: data.agent_id,
      metric: '',
      threshold: 0,
      actual_value: 0,
      severity: data.severity,
      message: data.message,
      status: data.status || 'firing',
      fired_at: data.fired_at,
      resolved_at: 0,
      notified_at: 0,
      created_at: ''
    }
    records.value.unshift(synthetic)
  })
})

onBeforeUnmount(() => {
  unsubAlert?.()
  window.removeEventListener('app:refresh', loadRecords)
})
</script>

<template>
  <div>
    <div class="pointer-events-none -mx-4 -mt-6 mb-6 h-px bg-gradient-to-r from-transparent via-warning/70 to-transparent sm:-mx-6"></div>

    <!-- 统计卡 -->
    <div class="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
      <StatCard label="今日告警" :value="todayRecords.length" tone="warning" />
      <StatCard label="触发中" :value="firingRecords.length" tone="danger" :pulse="firingRecords.length > 0" />
      <StatCard label="严重告警" :value="criticalRecords.length" tone="danger" />
      <StatCard label="平均持续时长" :value="avgDuration" tone="accent" />
    </div>

    <!-- 筛选器 -->
    <div class="mb-4 grid grid-cols-1 gap-2.5 sm:grid-cols-2 lg:grid-cols-4">
      <select v-model="agentFilter" class="form-input py-2 text-xs" aria-label="按 Agent 筛选">
        <option value="">全部 Agent</option>
        <option v-for="a in agents" :key="a.id" :value="a.id">{{ a.hostname }}</option>
      </select>
      <select v-model="severityFilter" class="form-input py-2 text-xs" aria-label="按严重级别筛选">
        <option value="">全部级别</option>
        <option value="warning">警告</option>
        <option value="critical">严重</option>
      </select>
      <select v-model="statusFilter" class="form-input py-2 text-xs" aria-label="按状态筛选">
        <option value="">全部状态</option>
        <option value="firing">触发中</option>
        <option value="resolved">已恢复</option>
      </select>
      <select v-model="rangeKey" class="form-input py-2 text-xs" aria-label="时间范围">
        <option value="24h">最近 24 小时</option>
        <option value="7d">最近 7 天</option>
        <option value="30d">最近 30 天</option>
      </select>
    </div>

    <!-- 记录表格 -->
    <div class="card overflow-hidden">
      <EmptyState
        v-if="records.length === 0 && !loading"
        variant="alert"
        text="暂无告警记录"
        hint="当前筛选条件下没有记录，系统运行良好"
      />
      <template v-if="records.length > 0">
        <div class="overflow-x-auto">
          <table class="data-table stack">
            <thead>
              <tr>
                <SortableTh field="fired_at" :sort-field="sortField" :sort-dir="sortDir" sortable @sort="onSort">时间</SortableTh>
                <SortableTh field="agent_id" :sort-field="sortField" :sort-dir="sortDir" sortable @sort="onSort">Agent</SortableTh>
                <SortableTh field="severity" :sort-field="sortField" :sort-dir="sortDir" sortable @sort="onSort">严重级别</SortableTh>
                <th>消息</th>
                <SortableTh field="status" :sort-field="sortField" :sort-dir="sortDir" sortable @sort="onSort">状态</SortableTh>
                <th>持续时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="record in pagedRecords" :key="record.id">
                <span class="row-indicator"></span>
                <td data-label="时间">
                  <span class="text-xs">
                    <p class="text-content">{{ formatRelativeTime(record.fired_at) }}</p>
                    <p class="text-[11px] text-muted">{{ formatDateTime(record.fired_at) }}</p>
                  </span>
                </td>
                <td data-label="Agent">{{ agentHostname(record.agent_id) }}</td>
                <td data-label="严重级别"><SeverityBadge :severity="record.severity" /></td>
                <td data-label="消息">
                  <p class="max-w-md truncate text-xs" :title="record.message">{{ record.message }}</p>
                </td>
                <td data-label="状态">
                  <span
                    class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="record.status === 'firing' ? 'bg-danger/15 text-danger' : 'bg-success/15 text-success'"
                  >
                    <span class="h-1.5 w-1.5 rounded-full" :class="record.status === 'firing' ? 'animate-pulse bg-danger' : 'bg-success'"></span>
                    {{ record.status === 'firing' ? '触发中' : '已恢复' }}
                  </span>
                </td>
                <td data-label="持续时间">
                  <span class="font-mono text-xs">{{ formatDuration(record.fired_at, record.resolved_at) }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="px-4">
          <PaginationBar v-model:page="page" v-model:page-size="pageSize" :total="sortedRecords.length" />
        </div>
      </template>
    </div>
  </div>
</template>
