/**
 * 全局通知（Toast）状态管理。
 * 用于展示 WebSocket 告警推送和操作反馈。
 */
import { defineStore } from 'pinia'
import { ref } from 'vue'

export type ToastType = 'info' | 'success' | 'warning' | 'danger'

export interface Toast {
  id: number
  type: ToastType
  title: string
  message?: string
}

let seq = 0
const DEFAULT_DURATION = 6000

export const useToastStore = defineStore('toast', () => {
  const toasts = ref<Toast[]>([])

  function push(type: ToastType, title: string, message?: string): number {
    const id = ++seq
    toasts.value.unshift({ id, type, title, message })

    // 自动移除
    setTimeout(() => dismiss(id), DEFAULT_DURATION)
    return id
  }

  function dismiss(id: number): void {
    toasts.value = toasts.value.filter((t) => t.id !== id)
  }

  return {
    toasts,
    push,
    dismiss,
    info: (title: string, message?: string) => push('info', title, message),
    success: (title: string, message?: string) => push('success', title, message),
    warning: (title: string, message?: string) => push('warning', title, message),
    danger: (title: string, message?: string) => push('danger', title, message)
  }
})
