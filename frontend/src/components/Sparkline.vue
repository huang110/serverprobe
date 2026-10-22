<script setup lang="ts">
/**
 * 迷你折线图（sparkline）。
 * 纯 SVG 实现，体积小、适合在卡片中重复渲染趋势。
 */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    data: number[]
    color?: string
    width?: number
    height?: number
    /** 固定最大值（如 100），不填则自适应 */
    max?: number
  }>(),
  {
    color: 'rgb(var(--success))',
    width: 96,
    height: 26
  }
)

const geometry = computed(() => {
  const values = props.data.length ? props.data : [0, 0]
  const max = props.max ?? Math.max(1, ...values)
  const min = props.max !== undefined ? 0 : Math.min(0, ...values)
  const range = max - min || 1
  const stepX = props.width / Math.max(1, values.length - 1)

  const points = values.map((v, i) => {
    const x = i * stepX
    const y = props.height - ((v - min) / range) * (props.height - 2) - 1
    return [Number(x.toFixed(2)), Number(y.toFixed(2))]
  })

  const line = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${p[0]},${p[1]}`).join(' ')
  const area = `${line} L${props.width},${props.height} L0,${props.height} Z`
  const last = points[points.length - 1]

  return { line, area, last }
})

const gradientId = `spark-${Math.random().toString(36).slice(2, 9)}`
</script>

<template>
  <svg
    :width="width"
    :height="height"
    :viewBox="`0 0 ${width} ${height}`"
    fill="none"
    aria-hidden="true"
  >
    <defs>
      <linearGradient :id="gradientId" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" :stop-color="color" stop-opacity="0.3" />
        <stop offset="100%" :stop-color="color" stop-opacity="0" />
      </linearGradient>
    </defs>
    <path :d="geometry.area" :fill="`url(#${gradientId})`" />
    <path
      :d="geometry.line"
      :stroke="color"
      stroke-width="1.5"
      stroke-linejoin="round"
      stroke-linecap="round"
    />
    <circle
      :cx="geometry.last[0]"
      :cy="geometry.last[1]"
      r="2"
      :fill="color"
    />
  </svg>
</template>
