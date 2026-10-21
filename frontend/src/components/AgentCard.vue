<script setup lang="ts">
/**
 * Agent 卡片组件（参考阿修罗监控卡片风格）。
 * - 展示主机名、状态、IP 及实时 CPU/内存/负载/上下行速率
 * - 数据更新时对应数值闪烁
 * - 离线时指标置灰显示 "--"
 */
import { computed, ref, watch } from 'vue'
import type { Agent } from '@/types/agent'
import type { RealtimeMetrics } from '@/types/metrics'
import StatusBadge from './StatusBadge.vue'
import { formatRate } from '@/utils/format'

const props = defineProps<{
  agent: Agent
  metrics?: RealtimeMetrics
}>()

const isOnline = computed(() => props.agent.status === 'online')
const m = computed(() => props.metrics)

// 记录各指标是否需要播放闪烁动画
const flashing = ref<Record<string, boolean>>({})

function triggerFlash(keys: string[]) {
  keys.forEach((key) => {
    flashing.value[key] = false
    // 下一帧重新触发动画
    requestAnimationFrame(() => {
      flashing.value[key] = true
    })
  })
}

watch(
  () => [m.value?.cpuUsage, m.value?.memoryPercent, m.value?.load1, m.value?.netRxBytes, m.value?.netTxBytes],
  (next, prev) => {
    if (!prev || !isOnline.value) return
    const keys = ['cpu', 'memory', 'load', 'rx', 'tx']
    triggerFlash(next.map((_, i) => (next[i] !== prev[i] ? keys[i] : '')).filter(Boolean))
  }
)

const cpuText = computed(() => (isOnline.value && m.value ? `${m.value.cpuUsage.toFixed(1)}%` : '--'))
const memText = computed(() => (isOnline.value && m.value ? `${m.value.memoryPercent.toFixed(1)}%` : '--'))
const loadText = computed(() => (isOnline.value && m.value ? m.value.load1.toFixed(2) : '--'))
const rxText = computed(() => (isOnline.value && m.value ? formatRate(m.value.netRxBytes) : '--'))
const txText = computed(() => (isOnline.value && m.value ? formatRate(m.value.netTxBytes) : '--'))

// CPU / 内存进度条宽度
const cpuWidth = computed(() => (m.value ? Math.min(100, m.value.cpuUsage) : 0))
const memWidth = computed(() => (m.value ? Math.min(100, m.value.memoryPercent) : 0))
</script>

<template>
  <div
    data-testid="agent-card"
    class="card group relative cursor-pointer overflow-hidden p-4 transition duration-200 hover:-translate-y-1 hover:border-accent/60 hover:shadow-lg hover:shadow-black/40"
  >
    <!-- 在线状态左侧高亮条 -->
    <span
      class="absolute inset-y-0 left-0 w-1"
      :class="isOnline ? 'bg-success/70' : 'bg-slate-600'"
    ></span>

    <!-- 标题行 -->
    <div class="mb-3 flex items-start justify-between gap-2 pl-1.5">
      <div class="min-w-0">
        <h3 class="truncate text-sm font-bold text-slate-100 group-hover:text-accent">
          {{ agent.hostname }}
        </h3>
        <p class="mt-0.5 truncate font-mono text-xs text-muted">{{ agent.ip }}</p>
      </div>
      <StatusBadge :status="agent.status" />
    </div>

    <!-- 指标列表 -->
    <div class="space-y-2 pl-1.5" :class="!isOnline && 'opacity-60'">
      <!-- CPU -->
      <div>
        <div class="flex items-center justify-between text-xs">
          <span class="text-muted">CPU</span>
          <span
            class="font-mono font-medium text-slate-200"
            :class="flashing.cpu && 'animate-flash rounded px-1'"
          >{{ cpuText }}</span>
        </div>
        <div class="mt-1 h-1 overflow-hidden rounded-full bg-ink-600">
          <div class="h-full rounded-full bg-success transition-all duration-500" :style="{ width: cpuWidth + '%' }"></div>
        </div>
      </div>

      <!-- 内存 -->
      <div>
        <div class="flex items-center justify-between text-xs">
          <span class="text-muted">内存</span>
          <span
            class="font-mono font-medium text-slate-200"
            :class="flashing.memory && 'animate-flash rounded px-1'"
          >{{ memText }}</span>
        </div>
        <div class="mt-1 h-1 overflow-hidden rounded-full bg-ink-600">
          <div class="h-full rounded-full bg-info transition-all duration-500" :style="{ width: memWidth + '%' }"></div>
        </div>
      </div>

      <!-- 负载 -->
      <div class="flex items-center justify-between text-xs">
        <span class="text-muted">负载</span>
        <span
          class="font-mono font-medium text-slate-200"
          :class="flashing.load && 'animate-flash rounded px-1'"
        >{{ loadText }}</span>
      </div>

      <!-- 上下行 -->
      <div class="flex items-center justify-between gap-2 border-t border-line/60 pt-2 text-xs">
        <span class="flex items-center gap-1">
          <span class="text-muted">↓</span>
          <span
            class="font-mono text-success"
            :class="flashing.rx && 'animate-flash rounded px-1'"
          >{{ rxText }}</span>
        </span>
        <span class="flex items-center gap-1">
          <span class="text-muted">↑</span>
          <span
            class="font-mono text-info"
            :class="flashing.tx && 'animate-flash rounded px-1'"
          >{{ txText }}</span>
        </span>
      </div>
    </div>
  </div>
</template>
