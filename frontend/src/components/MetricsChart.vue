<script setup lang="ts">
/**
 * ECharts 图表容器组件。
 * - 懒加载初始化，响应式更新配置
 * - 自动监听窗口大小变化进行 resize
 */
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'

const props = withDefaults(
  defineProps<{
    option: EChartsOption
    height?: string
    /** 无数据时显示空状态遮罩（图表容器仍保留，便于占位与测试选择） */
    empty?: boolean
    emptyText?: string
    /** E2E / 测试用的 data-testid，默认 metrics-chart；TCP Ping 图传 latency-chart */
    testId?: string
  }>(),
  {
    testId: 'metrics-chart'
  }
)

const chartEl = ref<HTMLDivElement>()
let chart: echarts.ECharts | null = null
let resizeObserver: ResizeObserver | null = null

function render() {
  if (chart) chart.setOption(props.option, true)
}

onMounted(() => {
  if (!chartEl.value) return
  chart = echarts.init(chartEl.value)
  render()

  // 容器尺寸变化时自适应
  resizeObserver = new ResizeObserver(() => chart?.resize())
  resizeObserver.observe(chartEl.value)
})

watch(() => props.option, render, { deep: true })

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div class="relative w-full" :style="{ height: height || '300px' }">
    <div
      ref="chartEl"
      :data-testid="testId"
      class="w-full"
      :class="empty && 'opacity-0'"
      :style="{ height: '100%' }"
    ></div>
    <div
      v-if="empty"
      class="absolute inset-0 flex flex-col items-center justify-center gap-2 text-center"
    >
      <svg class="h-10 w-10 text-line" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.5">
        <path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z" />
      </svg>
      <p class="text-xs text-muted">{{ emptyText || '该时间范围暂无数据' }}</p>
    </div>
  </div>
</template>
