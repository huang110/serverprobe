<script setup lang="ts">
/** 通用弹窗：支持 ESC 关闭、焦点遮罩点击关闭 */
import { onBeforeUnmount, watch } from 'vue'

const props = defineProps<{
  show: boolean
  title?: string
  width?: string
}>()

const emit = defineEmits<{
  close: []
}>()

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.show) emit('close')
}

watch(
  () => props.show,
  (visible) => {
    if (visible) window.addEventListener('keydown', onKeydown)
    else window.removeEventListener('keydown', onKeydown)
  }
)

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="show"
        class="fixed inset-0 z-[90] flex items-center justify-center p-4"
        @click.self="emit('close')"
      >
        <div class="absolute inset-0 bg-black/60 backdrop-blur-sm"></div>
        <div
          role="dialog"
          aria-modal="true"
          :aria-label="title"
          class="relative z-10 w-full animate-fade-in rounded-2xl border border-line bg-surface shadow-2xl shadow-black/50"
          :style="{ maxWidth: width || '560px' }"
        >
          <div class="flex items-center justify-between border-b border-line px-5 py-4">
            <h3 class="text-base font-semibold text-content">{{ title }}</h3>
            <button
              aria-label="关闭弹窗"
              class="rounded-md p-1 text-muted transition hover:bg-surface-3 hover:text-content"
              @click="emit('close')"
            >
              <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
          <div class="max-h-[70vh] overflow-y-auto px-5 py-4">
            <slot />
          </div>
          <div v-if="$slots.footer" class="flex justify-end gap-2.5 border-t border-line px-5 py-4">
            <slot name="footer" />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: opacity 0.2s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
