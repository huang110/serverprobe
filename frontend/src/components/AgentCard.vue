<script setup lang="ts">
/**
 * Agent 探针卡片（MJJ 晒鸡风格）。
 * - 头部：发行版徽章 + 备注名/地区/线路/价格标签 + 硬件规格微徽章（1C · 1.9G · 20G）
 * - 核心指标：CPU（含 Steal 偷跑徽章）、内存 + Swap、实时速率 + 累计总流量 + Uptime
 * - 状态彩色边框；离线显示最后在线；IP 支持打码
 */
import { computed } from 'vue'
import type { Agent } from '@/types/agent'
import type { RealtimeMetrics } from '@/types/metrics'
import type { SparkPoint } from '@/stores/agent'
import StatusBadge from './StatusBadge.vue'
import Sparkline from './Sparkline.vue'
import UsageBar from './UsageBar.vue'
import DistroBadge from './DistroBadge.vue'
import { ripple as vRipple } from '@/directives/ripple'
import {
  formatBytes,
  formatMB,
  formatRate,
  formatRelativeTime,
  formatUptime
} from '@/utils/format'
import { displayIp } from '@/utils/ip'
import { usePrefsStore } from '@/stores/prefs'

const props = defineProps<{
  agent: Agent
  metrics?: RealtimeMetrics
  spark?: SparkPoint[]
  compact?: boolean
  /** 搜索关键字，用于在主机名上高亮匹配 */
  highlight?: string
  /** 资产标签（region / line / price / expire / name 等） */
  labels?: Record<string, string>
}>()

const prefsStore = usePrefsStore()

const isOnline = computed(() => props.agent.status === 'online')
const m = computed(() => props.metrics)
const tags = computed(() => props.labels || {})

// ---------- 展示信息 ----------
/** 备注名优先，否则用 hostname */
const displayName = computed(() => tags.value.name || props.agent.hostname)

const ipText = computed(() => displayIp(props.agent.ip, prefsStore.hideIp))

/** 硬件规格微徽章：1C · 1.9G · 20G + 架构 */
const specText = computed(() => {
  const cores = `${props.agent.cpu_cores || '?'}C`
  const mem = props.agent.memory_total ? compactSize(props.agent.memory_total) : '?G'
  const disk = props.agent.disk_total ? compactSize(props.agent.disk_total) : '?G'
  return `${cores} · ${mem} · ${disk}`
})

/** 规格用紧凑容量：取 formatMB 的数字+单位（1.9 GB -> 1.9G） */
function compactSize(mb: number): string {
  const text = formatMB(mb, mb >= 10240 ? 0 : 1)
  return text.replace(' ', '').replace(/B$/, '')
}

/** 到期剩余天数 */
const expireDays = computed<number | null>(() => {
  const raw = tags.value.expire
  if (!raw) return null
  const date = new Date(raw)
  if (Number.isNaN(date.getTime())) return null
  return Math.ceil((date.getTime() - Date.now()) / 86400000)
})

// ---------- 指标 ----------
const cpuSpark = computed(() => props.spark?.map((p) => p.cpu) || [])
const memSpark = computed(() => props.spark?.map((p) => p.memory) || [])

const stealValue = computed(() => m.value?.cpuSteal ?? 0)
const showSteal = computed(() => isOnline.value && stealValue.value >= 5)

const swapText = computed(() => {
  if (!m.value || !m.value.swapTotal) return 'Swap: OFF'
  return `Swap: ${(m.value.swapPercent ?? 0).toFixed(0)}%`
})

const totalTraffic = computed(() => {
  if (!m.value) return { down: '--', up: '--' }
  return {
    down: formatBytes(m.value.netRxTotal),
    up: formatBytes(m.value.netTxTotal)
  }
})

// 状态边框
const borderClass = computed(() => {
  if (!isOnline.value) return 'border-line'
  if ((m.value?.cpuUsage ?? 0) >= 90 || (m.value?.memoryPercent ?? 0) >= 90) {
    return 'border-danger animate-pulse-border'
  }
  return 'border-success/50'
})

const lastSeenText = computed(() =>
  isOnline.value ? '' : formatRelativeTime(props.agent.last_heartbeat_at)
)

// 主机名搜索高亮
const safeName = computed(() => {
  const kw = props.highlight?.trim()
  if (!kw) return ''
  const safe = kw.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  return displayName.value.replace(
    new RegExp(`(${safe})`, 'ig'),
    '<mark class="rounded bg-warning/30 px-0.5 text-content">$1</mark>'
  )
})

// 地区/线路/价格标签
const chips = computed(() => {
  const list: { text: string; cls: string }[] = []
  if (tags.value.region) list.push({ text: tags.value.region, cls: 'text-accent bg-accent/10' })
  if (tags.value.line) list.push({ text: tags.value.line, cls: 'text-info bg-info/10' })
  if (tags.value.price) list.push({ text: tags.value.price, cls: 'text-success bg-success/10' })
  return list
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
    <!-- 顶部渐变光条 -->
    <span
      class="absolute inset-x-0 top-0 h-0.5 bg-gradient-to-r from-transparent via-success/70 to-transparent opacity-0 transition-opacity duration-300 group-hover:opacity-100"
    ></span>

    <!-- 标题行 -->
    <div class="mb-2 flex items-start justify-between gap-2">
      <div class="min-w-0">
        <div class="flex items-center gap-1.5">
          <h3
            class="truncate text-sm font-bold text-content group-hover:text-accent"
            v-html="highlight ? safeName : displayName"
          ></h3>
          <DistroBadge :agent="agent" />
        </div>
        <p class="mt-0.5 truncate font-mono text-[11px] text-muted">
          {{ ipText }}
          <span v-if="labels && labels.name" class="text-muted/70">· {{ agent.hostname }}</span>
        </p>
      </div>
      <StatusBadge :status="agent.status" :pulse="false" />
    </div>

    <!-- 资产标签：地区 / 线路 / 价格 -->
    <div v-if="chips.length || expireDays !== null" class="mb-2 flex flex-wrap items-center gap-1">
      <span
        v-for="chip in chips"
        :key="chip.text"
        class="rounded px-1.5 py-0.5 text-[10px] font-medium leading-none"
        :class="chip.cls"
      >{{ chip.text }}</span>
      <span
        v-if="expireDays !== null"
        class="rounded px-1.5 py-0.5 text-[10px] font-medium leading-none"
        :class="expireDays < 0
          ? 'bg-danger/15 text-danger'
          : expireDays <= 30
            ? 'bg-warning/15 text-warning'
            : 'bg-surface-2 text-muted'"
        :title="'到期日 ' + (labels?.expire || '')"
      >{{ expireDays < 0 ? `已过期 ${-expireDays}天` : `剩 ${expireDays}天` }}</span>
    </div>

    <!-- 硬件规格微徽章 -->
    <div class="mb-2.5 flex items-center gap-1.5">
      <span class="rounded bg-surface-2 px-1.5 py-0.5 font-mono text-[10px] font-semibold text-content/80">
        {{ specText }}
      </span>
      <span class="font-mono text-[10px] text-muted">{{ agent.arch || '' }}</span>
    </div>

    <!-- 在线：核心指标 -->
    <template v-if="isOnline">
      <!-- CPU -->
      <div class="mb-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="flex items-center gap-1.5 text-muted">
            CPU
            <span
              v-if="showSteal"
              class="rounded bg-warning/20 px-1 py-px text-[9px] font-bold leading-tight text-warning"
              title="邻居偷跑 Steal（疑似超售）"
            >偷跑 Steal {{ stealValue.toFixed(0) }}%</span>
          </span>
          <span class="flex items-center gap-2">
            <Sparkline v-if="cpuSpark.length > 1" :data="cpuSpark" :max="100" :width="compact ? 60 : 72" :height="20" color="rgb(var(--success))" />
            <span class="w-12 text-right font-mono font-medium">{{ m ? m.cpuUsage.toFixed(1) + '%' : '--' }}</span>
          </span>
        </div>
        <UsageBar :value="m?.cpuUsage ?? 0" class="mt-1.5" />
      </div>

      <!-- 内存 + Swap -->
      <div class="mb-2.5">
        <div class="flex items-center justify-between text-xs">
          <span class="text-muted">内存 <span class="text-[10px] text-muted/80">{{ swapText }}</span></span>
          <span class="flex items-center gap-2">
            <Sparkline v-if="memSpark.length > 1" :data="memSpark" :max="100" :width="compact ? 60 : 72" :height="20" color="rgb(var(--info))" />
            <span class="w-12 text-right font-mono font-medium">{{ m ? m.memoryPercent.toFixed(1) + '%' : '--' }}</span>
          </span>
        </div>
        <UsageBar :value="m?.memoryPercent ?? 0" class="mt-1.5" />
      </div>

      <!-- 实时速率 -->
      <div class="flex items-center justify-between gap-2 border-t border-line/60 pt-2.5 text-xs">
        <span class="flex items-center gap-1.5">
          <svg class="h-3 w-3 animate-arrow-down text-success" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true">
            <path d="M6 10 L1 4h3V1h4v3h3z" />
          </svg>
          <span class="font-mono text-success">{{ m ? formatRate(m.netRxBytes) : '--' }}</span>
        </span>
        <span class="flex items-center gap-1.5">
          <span class="font-mono text-info">{{ m ? formatRate(m.netTxBytes) : '--' }}</span>
          <svg class="h-3 w-3 animate-arrow-up text-info" viewBox="0 0 12 12" fill="currentColor" aria-hidden="true">
            <path d="M6 2 L1 8h3v3h4V8h3z" />
          </svg>
        </span>
      </div>

      <!-- 累计总流量 + Uptime -->
      <div class="mt-2 flex items-center justify-between gap-2 text-[10px]">
        <span class="truncate font-mono text-muted" title="累计已用总流量">
          总 ↓ <span class="text-content/80">{{ totalTraffic.down }}</span>
          · ↑ <span class="text-content/80">{{ totalTraffic.up }}</span>
        </span>
        <span class="shrink-0 font-mono text-muted" title="系统在线时长">
          Uptime {{ m && m.uptime ? formatUptime(m.uptime) : '--' }}
        </span>
      </div>
    </template>

    <!-- 离线：最后在线时间 -->
    <div v-else class="rounded-lg border border-line/70 bg-surface-2/60 px-3 py-4 text-center">
      <p class="text-xs text-muted">最后在线</p>
      <p class="mt-1 text-sm font-medium text-content">{{ lastSeenText }}</p>
    </div>
  </div>
</template>
