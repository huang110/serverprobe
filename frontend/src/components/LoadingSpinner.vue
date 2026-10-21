<script setup lang="ts">
/** 加载中动画 */
defineProps<{
  text?: string
}>()
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-3 py-16 text-muted">
    <span class="h-8 w-8 animate-spin rounded-full border-2 border-line border-t-accent"></span>
    <span class="text-sm">{{ text || '加载中...' }}</span>
  </div>
</template>
