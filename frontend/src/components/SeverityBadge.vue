<script setup lang="ts">
/** 严重级别徽章 */
import { computed } from 'vue'

const props = defineProps<{ severity?: string }>()

const config = computed(() => {
  switch (props.severity) {
    case 'critical':
      return { text: '严重', cls: 'bg-danger/15 text-danger border-danger/40' }
    case 'warning':
      return { text: '警告', cls: 'bg-warning/15 text-warning border-warning/40' }
    default:
      return { text: '信息', cls: 'bg-info/15 text-info border-info/40' }
  }
})
</script>

<template>
  <span
    class="inline-flex items-center gap-1 rounded-full border px-2 py-0.5 text-xs font-medium"
    :class="config.cls"
  >
    {{ config.text }}
  </span>
</template>
