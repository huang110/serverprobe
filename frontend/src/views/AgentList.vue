<script setup lang="ts">
/**
 * Agent 列表页：卡片网格 + 在线统计 + 搜索 + WebSocket 实时更新。
 */
import { computed, onMounted, ref } from 'vue'
import { storeToRefs } from 'pinia'
import AgentCard from '@/components/AgentCard.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import EmptyState from '@/components/EmptyState.vue'
import { useAgentStore } from '@/stores/agent'
import { useWebSocketStore } from '@/stores/websocket'

const agentStore = useAgentStore()
const wsStore = useWebSocketStore()
const { agents, metricsMap, loading, total, onlineCount } = storeToRefs(agentStore)

const keyword = ref('')
const statusFilter = ref<'all' | 'online' | 'offline'>('all')

// 建立 WebSocket 连接（刷新页面直接进入时兜底）
onMounted(() => {
  wsStore.connect()
})

async function loadData() {
  await agentStore.fetchAgents(keyword.value.trim())
  await agentStore.fetchAllRealtime()
}

function handleRefresh() {
  loadData()
}

const filteredAgents = computed(() => {
  if (statusFilter.value === 'all') return agents.value
  return agents.value.filter((a) => a.status === statusFilter.value)
})
</script>

<template>
  <div>
    <!-- 工具栏 -->
    <div class="mb-6 flex flex-wrap items-center gap-3">
      <div>
        <h1 class="text-xl font-bold text-white">Agent 列表</h1>
        <p class="mt-0.5 text-xs text-muted">
          在线
          <span class="font-semibold text-success">{{ onlineCount }}</span>
          / 总数
          <span class="font-semibold text-slate-300">{{ total }}</span>
        </p>
      </div>

      <div class="ml-auto flex flex-wrap items-center gap-2">
        <!-- 搜索框 -->
        <div class="relative">
          <svg class="pointer-events-none absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 21l-5.197-5.197m0 0A7.5 7.5 0 105.196 5.196a7.5 7.5 0 0010.607 10.607z" />
          </svg>
          <input
            v-model="keyword"
            type="text"
            class="form-input w-44 py-1.5 pl-8 text-xs"
            placeholder="搜索主机名 / IP"
            @keyup.enter="loadData"
          />
        </div>

        <!-- 状态筛选 -->
        <div class="flex rounded-lg border border-line bg-ink-700 p-0.5 text-xs">
          <button
            v-for="opt in (['all', 'online', 'offline'] as const)"
            :key="opt"
            class="rounded-md px-2.5 py-1 font-medium transition"
            :class="statusFilter === opt ? 'bg-accent text-white' : 'text-muted hover:text-slate-200'"
            @click="statusFilter = opt"
          >
            {{ opt === 'all' ? '全部' : opt === 'online' ? '在线' : '离线' }}
          </button>
        </div>

        <!-- 刷新 -->
        <button class="btn-ghost px-2.5 py-1.5" title="刷新" @click="handleRefresh">
          <svg class="h-4 w-4" :class="loading && 'animate-spin'" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
          </svg>
        </button>
      </div>
    </div>

    <!-- 加载中 -->
    <LoadingSpinner v-if="loading && agents.length === 0" />

    <!-- 空状态 -->
    <EmptyState
      v-else-if="filteredAgents.length === 0"
      text="暂无 Agent"
      hint="请确认 Agent 已成功注册，或调整搜索条件"
    />

    <!-- 卡片网格 -->
    <div v-else class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
      <router-link v-for="agent in filteredAgents" :key="agent.id" :to="`/agents/${agent.id}`">
        <AgentCard :agent="agent" :metrics="metricsMap[agent.id]" />
      </router-link>
    </div>
  </div>
</template>
