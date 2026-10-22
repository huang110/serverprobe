/**
 * 全局快捷键：
 * - "/"   聚焦页面主搜索框（通过 data-shortcut="search" 标记）
 * - "r"   刷新当前页面数据（触发 window 上的 app:refresh 事件）
 * - Escape 关闭已打开的弹窗（由 BaseModal 自行处理）/ 失焦
 * 输入框、文本域、下拉框内不触发 r 与 /。
 */
import { onBeforeUnmount, onMounted } from 'vue'

function isEditable(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null
  if (!el) return false
  const tag = el.tagName
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable
}

export function useShortcuts() {
  function onKeydown(event: KeyboardEvent) {
    if (event.key === '/' && !isEditable(event.target)) {
      const search = document.querySelector<HTMLElement>('[data-shortcut="search"]')
      if (search) {
        event.preventDefault()
        search.focus()
        search.dispatchEvent(new CustomEvent('shortcut:focus'))
      }
    } else if ((event.key === 'r' || event.key === 'R') && !isEditable(event.target)) {
      // 忽略带修饰键的情况（如 Ctrl+R 浏览器刷新）
      if (event.ctrlKey || event.metaKey || event.altKey) return
      event.preventDefault()
      window.dispatchEvent(new CustomEvent('app:refresh'))
    }
  }

  onMounted(() => window.addEventListener('keydown', onKeydown))
  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
}
