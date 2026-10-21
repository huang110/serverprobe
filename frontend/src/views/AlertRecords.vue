<script setup lang="ts">
/**
 * 告警记录页。
 * 后端按时间范围返回（最多 100 条）的记录数组，这里在客户端做分页；
 * WebSocket 收到新告警时在顶部插入，并弹出全局通知。
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import EmptyState from '@/components/EmptyState.vue'
import SeverityBadge from '@/components/SeverityBadge.vue'
import PaginationBar from '@/components/PaginationBar.vue'
import { listRecords } from '@/api/alert'
import { listAllAgents } from '@/api/agent'
import { wsClient } from '@/utils/websocket'
import {
  formatDateTime,
  formatDuration,
  formatRelativeTime
} from '@/utils/format'
import { useToastStore } from '@/stores/toast'
import type { AlertRecord } from '@/types/alert'
import type { Agent } from '@/types/agent'
import type { AlertMessage } from '@/types/metrics'

const toastStore = useToastStore()

const records = ref<AlertRecord[]>([])
const agents = ref<Agent[]>([])
const loading = ref(false)

// 筛选条件
const agentFilter = ref('')
const severityFilter = ref('')
const statusFilter = ref('')
const rangeKey = ref<'24h' | '7d' | '30d'>('24h')
const rangeOptions = [
  { key: '24h', label: '最近 24 小时', seconds: 86400 },
  { key: '7d', label: '最近 7 天', seconds: 604800 },
  { key: '30d', label: '最近 30 天', seconds: 2592000 }
] as const

// 分页
const page = ref(1)
const pageSize = ref(20)

async function loadRecords() {
  loading.value = true
  try {
    const range = rangeOptions.find((r) => r.key === rangeKey.value)!
    const end = Math.floor(Date.now() / 1000)
    const start = end - range.seconds

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

function agentHostname(agentId: string): string {
  return agents.value.find((a) => a.id === agentId)?.hostname || agentId.slice(0, 8)
}

function reload() {
  page.value = 1
  loadRecords()
}

// 筛选条件变化时重新加载
watch([agentFilter, severityFilter, statusFilter, rangeKey], reload)

// 客户端分页后的数据
const pagedRecords = computed(() => {
  const start = (page.value - 1) * pageSize.value
  return records.value.slice(start, start + pageSize.value)
})

// WebSocket 新告警：插入到列表顶部
let unsubAlert: (() => void) | null = null
onMounted(async () => {
  try {
    agents.value = await listAllAgents()
  } catch {
    /* 忽略 Agent 列表加载失败 */
  }
  await loadRecords()

  unsubAlert = wsClient.on('alert', (data: AlertMessage) => {
    const synthetic: AlertRecord = {
      id: Number(`${data.rule_id}${data.fired_at}`) || Date.now(),
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
    // 避免重复插入
    if (!records.value.some((r) => r.id === synthetic.id)) {
      records.value.unshift(synthetic)
    }
  })
})

onBeforeUnmount(() => unsubAlert?.())
</script>

<template>
  <div>
    <!-- 工具栏 -->
    <div class="mb-6 flex flex-wrap items-center gap-3">
      <div>
        <h1 class="text-xl font-bold text-white">告警记录</h1>
        <p class="mt-0.5 text-xs text-muted">查看历史触发与恢复情况</p>
      </div>
      <button class="btn-ghost ml-auto px-3 py-1.5 text-xs" @click="loadRecords">刷新</button>
    </div>

    <!-- 筛选器 -->
    <div class="mb-4 grid grid-cols-2 gap-2.5 sm:grid-cols-4">
      <select v-model="agentFilter" class="form-input py-2 text-xs">
        <option value="">全部 Agent</option>
        <option v-for="a in agents" :key="a.id" :value="a.id">{{ a.hostname }}</option>
      </select>
      <select v-model="severityFilter" class="form-input py-2 text-xs">
        <option value="">全部级别</option>
        <option value="warning">警告</option>
        <option value="critical">严重</option>
      </select>
      <select v-model="statusFilter" class="form-input py-2 text-xs">
        <option value="">全部状态</option>
        <option value="firing">触发中</option>
        <option value="resolved">已恢复</option>
      </select>
      <select v-model="rangeKey" class="form-input py-2 text-xs">
        <option v-for="r in rangeOptions" :key="r.key" :value="r.key">{{ r.label }}</option>
      </select>
    </div>

    <!-- 记录表格 -->
    <div class="card overflow-hidden">
      <LoadingSpinner v-if="loading" />
      <EmptyState
        v-else-if="records.length === 0"
        text="暂无告警记录"
        hint="当前筛选条件下没有记录，系统运行良好"
      />
      <template v-else>
        <div class="overflow-x-auto">
          <table class="data-table">
            <thead>
              <tr>
                <th>时间</th>
                <th>Agent</th>
                <th>严重级别</th>
                <th>消息</th>
                <th>状态</th>
                <th>持续时间</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="record in pagedRecords" :key="record.id">
                <td class="whitespace-nowrap">
                  <p class="text-xs text-slate-200">{{ formatRelativeTime(record.fired_at) }}</p>
                  <p class="text-[11px] text-muted">{{ formatDateTime(record.fired_at) }}</p>
                </td>
                <td class="whitespace-nowrap">{{ agentHostname(record.agent_id) }}</td>
                <td><SeverityBadge :severity="record.severity" /></td>
                <td class="max-w-md">
                  <p class="truncate text-xs text-slate-300" :title="record.message">{{ record.message }}</p>
                </td>
                <td>
                  <span
                    class="inline-flex items-center gap-1.5 rounded-full px-2 py-0.5 text-xs font-medium"
                    :class="
                      record.status === 'firing'
                        ? 'bg-danger/15 text-danger'
                        : 'bg-success/15 text-success'
                    "
                  >
                    <span class="h-1.5 w-1.5 rounded-full" :class="record.status === 'firing' ? 'animate-pulse bg-danger' : 'bg-success'"></span>
                    {{ record.status === 'firing' ? '触发中' : '已恢复' }}
                  </span>
                </td>
                <td class="whitespace-nowrap font-mono text-xs">
                  {{ formatDuration(record.fired_at, record.resolved_at) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="px-4">
          <PaginationBar v-model:page="page" v-model:page-size="pageSize" :total="records.length" />
        </div>
      </template>
    </div>
  </div>
</template>
