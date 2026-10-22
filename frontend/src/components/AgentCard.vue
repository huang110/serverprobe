<script setup lang="ts">
/**
 * Agent 卡片。
 * - 状态彩色边框：在线绿 / 离线灰 / 告警红色脉动
 * - 实时指标 + sparkline 趋势
 * - 网络上下行动态箭头
 * - 离线时显示最后在线时间
 * - 根据密度与自定义指标偏好调整
 */
import { computed, ref, watch } from 'vue'
import type { Agent } from '@/types/agent'
import type { RealtimeMetrics } from '@/types/metrics'
import type { SparkPoint } from '@/stores/agent'
import StatusBadge from './StatusBadge.vue'
import Sparkline from './Sparkline.vue'
import UsageBar from './UsageBar.vue'
import { ripple as vRipple } from '@/directives/ripple'
import { formatRate, formatRelativeTime } from '@/utils/format'
import { usePrefsStore } from '@/stores/prefs'

const props = defineProps<{
  agent: Agent
  metrics?: RealtimeMetrics
  spark?: SparkPoint[]
  compact?: boolean
  /** 搜索关键字，用于在主机名上高亮匹配 */
  highlight?: string
}>()

const prefsStore = usePrefsStore()

/** 用户自定义卡片指标：某项是否显示（至少保留一项） */
function showMetric(key: string): boolean {
  return prefsStore.cardMetrics.includes(key)
}

/** 网络行上方是否还有其他指标（决定是否显示分隔线） */
const hasMetricAbove = computed(() =>
  ['cpu', 'memory', 'load'].some((k) => prefsStore.cardMetrics.includes(k))
)

const isOnline = computed(() => props.agent.status === 'online')
const m = computed(() => props.metrics)

const flashing = ref<Record<string, boolean>>({})

watch(
  () => [m.value?.cpuUsage, m.value?.memoryPercent, m.value?.load1],
  (next, prev) => {
    if (!prev || !isOnline.value) return
    const keys = ['cpu', 'memory', 'load']
    next.forEach((v, i) => {
      if (v !== prev[i]) {
        flashing.value[keys[i]] = false
        requestAnimationFrame(() => (flashing.value[keys[i]] = true))
      }
    })
  }
)

// 状态边框
const borderClass = computed(() => {
  if (!isOnline.value) return 'border-line'
  // 高负载视为告警态：红色脉动边框
  if ((m.value?.cpuUsage ?? 0) >= 90 || (m.value?.memoryPercent ?? 0) >= 90) {
    return 'border-danger animate-pulse-border'
  }
  return 'border-success/50'
})

const cpuSpark = computed(() => props.spark?.map((p) => p.cpu) || [])
const memSpark = computed(() => props.spark?.map((p) => p.memory) || [])

const lastSeenText = computed(() =>
  isOnline.value ? '' : formatRelativeTime(props.agent.last_heartbeat_at)
)

const loadRatio = computed(() =>
  m.value ? m.value.load1 / Math.max(1, props.agent.cpu_cores) : 0
)
const overload = computed(() => loadRatio.value > 1)

// 主机名搜索高亮
const safeHostname = computed(() => {
  const kw = props.highlight?.trim()
  if (!kw) return ''
  const safe = kw.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return props.agent.hostname.replace(
    new RegExp(`(${safe})`, 'ig'),
    '<mark class="rounded bg-warning/30 px-0.5 text-content">$1</mark>'
  )
})
</script>

<template>
  <div
    v-ripple
    data-testid="agent-card"
    class="card group relative h-full cursor-pointer border transition-all duration-200 hover:-translate-y-1 hover:shadow-lg hover:shadow-black/40"
    :class="borderClass"
    :style="{ padding: 'var(--density-y) var(--density-x)' }"
  >
    <!-- 在线状态顶部渐变光条 -->
    <span
      class="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-accent/70 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"
      :class="isOnline && 'via-success/70'"
    ></span>

    <!-- 标题行 -->
    <div class="mb-3 flex items-start justify-between gap-2">
      <div class="min-w-0">
        <h3
          class="truncate text-sm font-bold text-content group-hover:text-accent"
          v-html="highlight ? safeHostname : agent.hostname"
        ></h3>
        <p class="mt-0.5 truncate font-mono text-xs text-muted">{{ agent.ip || '-' }}</p>
      </div>
      <StatusBadge :status="agent.status" :pulse="false" />
    </div>

    <!-- 在线：指标 -->
    <template v-if="isOnline">
      <!-- CPU -->
      <div v-if="showMetric('cpu')" class="mb-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="text-muted">CPU</span>
          <span class="flex items-center gap-2">
            <Sparkline v-if="cpuSpark.length > 1" :data="cpuSpark" :max="100" :width="compact ? 70 : 84" :height="22" color="rgb(var(--success))" />
            <span class="w-12 text-right font-mono font-medium" :class="flashing.cpu && 'animate-flash rounded'">{{ m ? m.cpuUsage.toFixed(1) + '%' : '--' }}</span>
          </span>
        </div>
        <UsageBar :value="m?.cpuUsage ?? 0" class="mt-1.5" />
      </div>

      <!-- 内存 -->
      <div v-if="showMetric('memory')" class="mb-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="text-muted">内存</span>
          <span class="flex items-center gap-2">
            <Sparkline v-if="memSpark.length > 1" :data="memSpark" :max="100" :width="compact ? 70 : 84" :height="22" color="rgb(var(--info))" />
            <span class="w-12 text-right font-mono font-medium" :class="flashing.memory && 'animate-flash rounded'">{{ m ? m.memoryPercent.toFixed(1) + '%' : '--' }}</span>
          </span>
        </div>
        <UsageBar :value="m?.memoryPercent ?? 0" class="mt-1.5" />
      </div>

      <!-- 负载 -->
      <div v-if="showMetric('load')" class="mb-2.5 flex items-center justify-between text-xs">
        <span class="text-muted">负载 (load1)</span>
        <span class="font-mono font-medium" :class="[flashing.load && 'animate-flash rounded', overload ? 'text-warning' : 'text-content']">
          {{ m ? m.load1.toFixed(2) : '--' }}
        </span>
      </div>

      <!-- 上下行 -->
      <div
        v-if="showMetric('network')"
        class="flex items-center justify-between gap-2 pt-2.5 text-xs"
        :class="hasMetricAbove && 'border-t border-line/60'"
      >
        <span class="flex items-center gap-1.5">
          <svg class="h-3 w-3 animate-arrow-down text-success" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true">
            <path d="M6 10 L1 4h3V1h4v3h3z" />
          </svg>
          <span class="font-mono text-success">{{ m ? formatRate(m.netRxBytes) : '--' }}</span>
        </span>
        <span class="flex items-center gap-1.5">
          <svg class="h-3 w-3 animate-arrow-up text-info" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true">
            <path d="M6 2 L1 8h3v3h4V8h3z" />
          </svg>
          <span class="font-mono text-info">{{ m ? formatRate(m.netTxBytes) : '--' }}</span>
        </span>
      </div>

      <!-- 所有指标均被关闭 -->
      <p v-if="prefsStore.cardMetrics.length === 0" class="py-3 text-center text-xs text-muted">
        已在偏好设置中隐藏全部指标
      </p>
    </template>

    <!-- 离线：最后在线时间 -->
    <div v-else class="rounded-lg border border-line/70 bg-surface-2/60 px-3 py-4 text-center">
      <p class="text-xs text-muted">最后在线</p>
      <p class="mt-1 text-sm font-medium text-content">{{ lastSeenText }}</p>
    </div>
  </div>
</template>
