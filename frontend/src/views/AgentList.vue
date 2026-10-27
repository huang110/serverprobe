<script setup lang="ts">
/**
 * 探针列表晒鸡大屏（MJJ 风格）。
 * - 顶部 KPI 条：在线节点、实时总带宽（↑/↓ 汇总）、累计总流量（↓/↑ 汇总）、触发中告警
 * - 详细 / 紧凑视图、防抖搜索 + 高亮、IP 打码开关
 * - 骨架屏、IntersectionObserver 分批渲染、WebSocket 实时更新
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import AgentCard from '@/components/AgentCard.vue'
import AgentCardSkeleton from '@/components/AgentCardSkeleton.vue'
import EmptyState from '@/components/EmptyState.vue'
import SegmentedControl from '@/components/SegmentedControl.vue'
import { useAgentStore } from '@/stores/agent'
import { useWebSocketStore } from '@/stores/websocket'
import { usePrefsStore } from '@/stores/prefs'
import { formatBytes, formatRate } from '@/utils/format'

const agentStore = useAgentStore()
const wsStore = useWebSocketStore()
const prefsStore = usePrefsStore()
const { agents, metricsMap, sparkMap, labelsMap, loading } = storeToRefs(agentStore)

// 搜索：300ms 防抖后请求后端，本地即时过滤
const keyword = ref('')
let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    agentStore.fetchAgents(keyword.value.trim())
  }, 300)
})

const statusFilter = ref<'all' | 'online' | 'offline'>('all')

onMounted(() => {
  wsStore.connect()
  loadData()
})

async function loadData() {
  await agentStore.fetchAgents(keyword.value.trim())
  await Promise.all([agentStore.fetchAllRealtime(), agentStore.fetchAllLabels()])
}

// 全局快捷键：R 刷新
function onRefresh() {
  loadData()
}
onMounted(() => window.addEventListener('app:refresh', onRefresh))
onBeforeUnmount(() => window.removeEventListener('app:refresh', onRefresh))

// 视图：详细 / 紧凑
const view = computed({
  get: () => prefsStore.viewMode,
  set: (v: 'grid' | 'compact') => (prefsStore.viewMode = v)
})

// 本地过滤
const filteredAgents = computed(() => {
  let list = agents.value
  if (statusFilter.value !== 'all') {
    list = list.filter((a) => a.status === statusFilter.value)
  }
  const kw = keyword.value.trim().toLowerCase()
  if (kw) {
    list = list.filter(
      (a) => a.hostname.toLowerCase().includes(kw) || a.ip.toLowerCase().includes(kw)
    )
  }
  return list
})

// ---------- 顶部 KPI 汇总 ----------
/** 在线机器的实时总带宽（Bytes/s） */
const totalBandwidth = computed(() => {
  let down = 0
  let up = 0
  agents.value.forEach((a) => {
    if (a.status !== 'online') return
    const mt = metricsMap.value[a.id]
    if (mt) {
      down += mt.netRxBytes
      up += mt.netTxBytes
    }
  })
  return { down, up }
})

/** 累计总流量（Bytes） */
const totalTraffic = computed(() => {
  let down = 0
  let up = 0
  agents.value.forEach((a) => {
    const mt = metricsMap.value[a.id]
    if (mt) {
      down += mt.netRxTotal
      up += mt.netTxTotal
    }
  })
  return { down, up }
})

// ---------- 无限渲染 ----------
const PAGE_SIZE = 12
const visibleCount = ref(PAGE_SIZE)
const sentinelRef = ref<HTMLElement | null>(null)
let observer: IntersectionObserver | null = null

onMounted(() => {
  observer = new IntersectionObserver(
    (entries) => {
      if (entries[0].isIntersecting) {
        visibleCount.value = Math.min(filteredAgents.value.length, visibleCount.value + PAGE_SIZE)
      }
    },
    { rootMargin: '200px' }
  )
  if (sentinelRef.value) observer.observe(sentinelRef.value)
})

watch([filteredAgents, () => statusFilter.value, keyword], () => {
  visibleCount.value = PAGE_SIZE
  nextTick(() => {
    if (sentinelRef.value) observer?.observe(sentinelRef.value)
  })
})

onBeforeUnmount(() => observer?.disconnect())

const visibleAgents = computed(() => filteredAgents.value.slice(0, visibleCount.value))
const hasMore = computed(() => visibleCount.value < filteredAgents.value.length)
</script>

<template>
  <div>
    <!-- 顶部渐变光条 -->
    <div class="pointer-events-none -mx-4 -mt-6 mb-5 h-px bg-gradient-to-r from-transparent via-accent/70 to-transparent sm:-mx-6"></div>

    <!-- KPI 汇总条 -->
    <div class="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4">
      <!-- 在线节点 -->
      <div class="card relative overflow-hidden p-3.5">
        <div class="pointer-events-none absolute -right-5 -top-5 h-16 w-16 rounded-full bg-success/15 blur-xl"></div>
        <p class="text-[11px] text-muted">在线节点</p>
        <p class="mt-1 font-mono text-xl font-bold leading-none text-success">
          {{ agentStore.onlineCount }}<span class="text-sm font-medium text-muted"> / {{ agentStore.total }}</span>
        </p>
      </div>

      <!-- 实时总带宽 -->
      <div class="card relative overflow-hidden p-3.5">
        <div class="pointer-events-none absolute -right-5 -top-5 h-16 w-16 rounded-full bg-accent/15 blur-xl"></div>
        <p class="text-[11px] text-muted">实时总带宽</p>
        <p class="mt-1 truncate font-mono text-[13px] font-bold leading-tight">
          <span class="text-success">↓ {{ formatRate(totalBandwidth.down, 1) }}</span>
        </p>
        <p class="truncate font-mono text-[13px] font-bold leading-tight">
          <span class="text-info">↑ {{ formatRate(totalBandwidth.up, 1) }}</span>
        </p>
      </div>

      <!-- 累计总流量 -->
      <div class="card relative overflow-hidden p-3.5">
        <div class="pointer-events-none absolute -right-5 -top-5 h-16 w-16 rounded-full bg-info/15 blur-xl"></div>
        <p class="text-[11px] text-muted">累计总流量</p>
        <p class="mt-1 truncate font-mono text-[13px] font-bold leading-tight">
          <span class="text-success">↓ {{ formatBytes(totalTraffic.down) }}</span>
        </p>
        <p class="truncate font-mono text-[13px] font-bold leading-tight">
          <span class="text-info">↑ {{ formatBytes(totalTraffic.up) }}</span>
        </p>
      </div>

      <!-- 触发中告警 -->
      <div class="card relative overflow-hidden p-3.5">
        <div class="pointer-events-none absolute -right-5 -top-5 h-16 w-16 rounded-full bg-danger/15 blur-xl"></div>
        <p class="text-[11px] text-muted">触发中告警</p>
        <p class="mt-1 flex items-center gap-1.5 font-mono text-xl font-bold leading-none"
          :class="wsStore.firingAlertCount > 0 ? 'text-danger' : 'text-content'">
          <span v-if="wsStore.firingAlertCount > 0" class="h-2 w-2 animate-ping rounded-full bg-danger"></span>
          {{ wsStore.firingAlertCount }}
        </p>
      </div>
    </div>

    <!-- 工具栏 -->
    <div class="mb-5 flex flex-wrap items-center gap-3">
      <div class="relative">
        <svg class="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
        </svg>
        <input
          v-model="keyword"
          data-shortcut="search"
          type="text"
          aria-label="搜索探针主机名或 IP"
          class="form-input w-48 py-1.5 pl-8 text-xs"
          placeholder="搜索主机名 / IP（按 / 聚焦）"
        />
      </div>

      <SegmentedControl
        v-model="statusFilter"
        size="sm"
        aria-label="按在线状态筛选"
        :options="[
          { value: 'all', label: '全部' },
          { value: 'online', label: '在线' },
          { value: 'offline', label: '离线' }
        ]"
      />

      <div class="ml-auto flex items-center gap-2">
        <!-- IP 打码开关 -->
        <button
          type="button"
          class="btn-ghost px-2.5 py-1.5 text-xs"
          :aria-pressed="prefsStore.hideIp"
          :aria-label="prefsStore.hideIp ? '显示 IP' : '隐藏 IP（截图打码）'"
          @click="prefsStore.hideIp = !prefsStore.hideIp"
        >
          <span aria-hidden="true">{{ prefsStore.hideIp ? '🙈' : '👁️' }}</span>
          <span class="hidden sm:inline">{{ prefsStore.hideIp ? '已隐藏' : '隐藏IP' }}</span>
        </button>

        <SegmentedControl
          v-model="view"
          size="sm"
          aria-label="切换视图密度"
          :options="[
            { value: 'grid', label: '详细' },
            { value: 'compact', label: '紧凑' }
          ]"
        />
        <button
          aria-label="刷新探针列表（快捷键 R）"
          class="btn-ghost px-2.5 py-1.5"
          @click="loadData"
        >
          <svg class="h-4 w-4" :class="loading && 'animate-spin'" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
          </svg>
        </button>
      </div>
    </div>

    <!-- 骨架屏 -->
    <div v-if="loading && agents.length === 0" class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <AgentCardSkeleton v-for="i in 8" :key="i" :compact="view === 'compact'" />
    </div>

    <!-- 空状态 -->
    <EmptyState
      v-else-if="filteredAgents.length === 0"
      :variant="keyword ? 'search' : 'agent'"
      :text="keyword ? `没有匹配「${keyword}」的探针` : '暂无探针节点'"
      :hint="keyword ? '尝试更换关键字' : '请确认 Agent 已成功注册'"
    />

    <!-- 卡片网格 -->
    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <router-link v-for="agent in visibleAgents" :key="agent.id" :to="`/agents/${agent.id}`" class="block">
        <AgentCard
          :agent="agent"
          :metrics="metricsMap[agent.id]"
          :spark="sparkMap[agent.id]"
          :compact="view === 'compact'"
          :highlight="keyword"
          :labels="labelsMap[agent.id]"
        />
      </router-link>
    </div>

    <!-- 无限加载哨兵 -->
    <div ref="sentinelRef" class="h-4"></div>
    <p v-if="hasMore && !loading" class="py-2 text-center text-xs text-muted">滚动加载更多…</p>
  </div>
</template>
