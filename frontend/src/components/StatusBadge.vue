<script setup lang="ts">
/** 在线/离线状态徽章（圆点 + 文案） */
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    status?: string
    onlineText?: string
    offlineText?: string
    pulse?: boolean
  }>(),
  {
    status: 'offline',
    onlineText: '在线',
    offlineText: '离线',
    pulse: true
  }
)

const isOnline = computed(() => props.status === 'online')
</script>

<template>
  <span class="inline-flex items-center gap-1.5 text-sm" :class="isOnline ? 'text-success' : 'text-muted'">
    <span class="relative flex h-2.5 w-2.5">
      <span
        v-if="isOnline && pulse"
        class="absolute inline-flex h-full w-full animate-ping rounded-full bg-success opacity-60"
      ></span>
      <span
        class="relative inline-flex h-2.5 w-2.5 rounded-full"
        :class="isOnline ? 'bg-success' : 'bg-muted'"
      ></span>
    </span>
    {{ isOnline ? onlineText : offlineText }}
  </span>
</template>
