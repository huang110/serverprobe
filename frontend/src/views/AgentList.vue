<script setup lang="ts">
/**
 * Agent 列表页。
 * - 顶部蓝色渐变条 + KPI（在线/总数、实时告警）
 * - 详细 / 紧凑视图切换（滑动指示条）
 * - 防抖搜索 + 高亮匹配
 * - 骨架屏加载、长列表窗口式渲染（IntersectionObserver 无限加载）
 * - WebSocket 实时更新
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { storeToRefs } from 'pinia'
import AgentCard from '@/components/AgentCard.vue'
import AgentCardSkeleton from '@/components/AgentCardSkeleton.vue'
import EmptyState from '@/components/EmptyState.vue'
import SegmentedControl from '@/components/SegmentedControl.vue'
import StatCard from '@/components/StatCard.vue'
import { useAgentStore } from '@/stores/agent'
import { useWebSocketStore } from '@/stores/websocket'
import { usePrefsStore } from '@/stores/prefs'

const agentStore = useAgentStore()
const wsStore = useWebSocketStore()
const prefsStore = usePrefsStore()
const { agents, metricsMap, sparkMap, loading } = storeToRefs(agentStore)

// 搜索：输入即时本地过滤，300ms 防抖后请求后端（重关键字场景）
const keyword = ref('')
let debounceTimer: ReturnType<typeof setTimeout> | null = null

watch(keyword, () => {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    agentStore.fetchAgents(keyword.value.trim())
  }, 300)
})

const statusFilter = ref<'all' | 'online' | 'offline'>('all')

onMounted(() => wsStore.connect())

async function loadData() {
  await agentStore.fetchAgents(keyword.value.trim())
  await agentStore.fetchAllRealtime()
}

// 全局快捷键：R 刷新
function onRefresh() {
  loadData()
}
onMounted(() => window.addEventListener('app:refresh', onRefresh))
onBeforeUnmount(() => window.removeEventListener('app:refresh', onRefresh))

// 视图切换：grid 详细 / compact 紧凑
const view = computed({
  get: () => prefsStore.viewMode,
  set: (v: 'grid' | 'compact') => (prefsStore.viewMode = v)
})

// 本地过滤（即时响应，无需等待后端）
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

// 无限渲染：首屏渲染一页，滚动到底部追加
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

// 过滤变化时重置窗口
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
    <div class="pointer-events-none -mx-4 -mt-6 mb-6 h-px bg-gradient-to-r from-transparent via-accent/70 to-transparent sm:-mx-6"></div>

    <!-- KPI 区 -->
    <div class="mb-6 grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      <StatCard
        label="在线 Agent"
        :value="agentStore.onlineCount"
        :sub="`/ ${agentStore.total} 台主机`"
        tone="success"
      />
      <StatCard
        label="离线 Agent"
        :value="agentStore.total - agentStore.onlineCount"
        sub="需要关注"
        :tone="agentStore.total - agentStore.onlineCount > 0 ? 'warning' : 'accent'"
        :pulse="agentStore.total - agentStore.onlineCount > 0"
      />
      <StatCard
        label="触发中告警"
        :value="wsStore.firingAlertCount"
        sub="实时推送"
        tone="danger"
        :pulse="wsStore.firingAlertCount > 0"
      />
      <div class="hidden lg:block">
        <StatCard label="监控主机总数" :value="agentStore.total" sub="全部已注册" tone="accent" />
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
          aria-label="搜索 Agent 主机名或 IP"
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
          aria-label="刷新 Agent 列表（快捷键 R）"
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
      :text="keyword ? `没有匹配「${keyword}」的 Agent` : '暂无 Agent'"
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
        />
      </router-link>
    </div>

    <!-- 无限加载哨兵 -->
    <div ref="sentinelRef" class="h-4"></div>
    <p v-if="hasMore && !loading" class="py-2 text-center text-xs text-muted">滚动加载更多…</p>
  </div>
</template>
