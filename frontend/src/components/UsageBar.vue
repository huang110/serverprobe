<script setup lang="ts">
/**
 * 使用率进度条：
 * - 根据阈值在 success -> warning -> danger 之间渐变过渡
 * - 宽度变化带平滑过渡动画
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value: number // 0-100
    showGlow?: boolean
    height?: string
  }>(),
  { showGlow: true, height: '4px' }
)

const clamped = computed(() => Math.max(0, Math.min(100, props.value || 0)))

// 超过 90% 增加危险脉动
const critical = computed(() => clamped.value >= 90)

const barStyle = computed(() => {
  // 在阈值处用渐变色平滑过渡
  let gradient: string
  if (clamped.value >= 90) {
    gradient = 'linear-gradient(90deg, rgb(var(--warning)), rgb(var(--danger)))'
  } else if (clamped.value >= 75) {
    gradient = 'linear-gradient(90deg, rgb(var(--success)), rgb(var(--warning)))'
  } else {
    gradient = 'linear-gradient(90deg, rgb(var(--success)), rgb(var(--info)))'
  }
  return {
    width: clamped.value + '%',
    background: gradient,
    height: props.height,
    boxShadow: props.showGlow && clamped.value >= 75 ? '0 0 8px -1px rgb(var(--warning) / 0.7)' : 'none'
  }
})
</script>

<template>
  <div
    class="w-full overflow-hidden rounded-full bg-surface-3"
    :style="{ height }"
    role="progressbar"
    :aria-valuenow="Math.round(clamped)"
    aria-valuemin="0"
    aria-valuemax="100"
  >
    <div
      class="h-full rounded-full transition-all duration-700 ease-out"
      :class="critical && 'animate-pulse'"
      :style="barStyle"
    ></div>
  </div>
</template>
