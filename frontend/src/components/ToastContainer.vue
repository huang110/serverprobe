<script setup lang="ts">
/** 全局通知容器，固定在右上角 */
import { useToastStore } from '@/stores/toast'

const toastStore = useToastStore()

const iconMap: Record<string, string> = {
  info: 'ℹ️',
  success: '✅',
  warning: '⚠️',
  danger: '🚨'
}

const borderMap: Record<string, string> = {
  info: 'border-info/50',
  success: 'border-success/50',
  warning: 'border-warning/50',
  danger: 'border-danger/50'
}
</script>

<template>
  <div class="pointer-events-none fixed right-4 top-4 z-[100] flex w-80 max-w-[calc(100vw-2rem)] flex-col gap-2">
    <TransitionGroup name="toast">
      <div
        v-for="toast in toastStore.toasts"
        :key="toast.id"
        class="pointer-events-auto animate-slide-in cursor-pointer rounded-xl border bg-ink-800/95 p-3.5 shadow-lg shadow-black/40 backdrop-blur"
        :class="borderMap[toast.type]"
        @click="toastStore.dismiss(toast.id)"
      >
        <div class="flex items-start gap-2.5">
          <span class="text-base leading-5">{{ iconMap[toast.type] }}</span>
          <div class="min-w-0 flex-1">
            <p class="text-sm font-semibold text-slate-100">{{ toast.title }}</p>
            <p v-if="toast.message" class="mt-0.5 break-words text-xs leading-relaxed text-muted">
              {{ toast.message }}
            </p>
          </div>
        </div>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toast-leave-active {
  transition: all 0.25s ease;
}
.toast-leave-to {
  opacity: 0;
  transform: translateX(24px);
}
</style>
