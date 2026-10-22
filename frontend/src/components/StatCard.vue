<script setup lang="ts">
/** 顶部 KPI 指标卡：渐变数字 + 发光强调 */
import { computed } from 'vue'

type Tone = 'accent' | 'success' | 'warning' | 'danger'

const props = withDefaults(
  defineProps<{
    label: string
    value: number | string
    sub?: string
    tone?: Tone
    pulse?: boolean
  }>(),
  { tone: 'accent', pulse: false }
)

const toneClass = computed(() => {
  switch (props.tone) {
    case 'success':
      return {
        glow: 'group-hover:shadow-glow-success',
        ring: 'from-success/20',
        gradient: 'from-emerald-400 to-emerald-600'
      }
    case 'warning':
      return {
        glow: 'group-hover:shadow-glow-danger',
        ring: 'from-warning/20',
        gradient: 'from-amber-400 to-orange-600'
      }
    case 'danger':
      return {
        glow: 'group-hover:shadow-glow-danger',
        ring: 'from-danger/25',
        gradient: 'from-rose-400 to-rose-600'
      }
    default:
      return {
        glow: 'group-hover:shadow-glow',
        ring: 'from-accent/20',
        gradient: 'from-sky-400 to-blue-600'
      }
  }
})
</script>

<template>
  <div
    class="card group relative overflow-hidden p-4 transition-shadow duration-300"
    :class="toneClass.glow"
  >
    <!-- 角落渐变光 -->
    <div
      class="pointer-events-none absolute -right-6 -top-6 h-20 w-20 rounded-full bg-gradient-to-br to-transparent blur-xl"
      :class="toneClass.ring"
    ></div>

    <div class="relative flex items-center gap-3">
      <span
        v-if="pulse"
        class="h-2.5 w-2.5 shrink-0 animate-ping rounded-full"
        :class="tone === 'danger' ? 'bg-danger' : tone === 'warning' ? 'bg-warning' : tone === 'success' ? 'bg-success' : 'bg-accent'"
      ></span>
      <div class="min-w-0">
        <p class="text-xs text-muted">{{ label }}</p>
        <p
          class="mt-0.5 bg-gradient-to-br bg-clip-text text-2xl font-bold leading-none text-transparent"
          :class="toneClass.gradient"
        >
          {{ value }}
        </p>
        <p v-if="sub" class="mt-1 truncate text-[11px] text-muted">{{ sub }}</p>
      </div>
    </div>
  </div>
</template>
