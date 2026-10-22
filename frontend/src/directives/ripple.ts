/**
 * v-ripple 指令：为元素点击添加涟漪效果。
 * 用法：<button v-ripple>或 <div v-ripple="'rgba(...)'">
 */
import type { Directive } from 'vue'

type RippleEl = HTMLElement & { __rippleHandler__?: (e: MouseEvent) => void }

export const ripple: Directive<HTMLElement, string | undefined> = {
  mounted(el: RippleEl, binding) {
    // 确保涟漪圆不溢出
    const computed = window.getComputedStyle(el)
    if (computed.position === 'static') el.style.position = 'relative'
    if (computed.overflow === 'visible') el.style.overflow = 'hidden'

    el.__rippleHandler__ = (event: MouseEvent) => {
      const rect = el.getBoundingClientRect()
      const size = Math.max(rect.width, rect.height)
      const span = document.createElement('span')
      span.style.position = 'absolute'
      span.style.borderRadius = '9999px'
      span.style.pointerEvents = 'none'
      span.style.width = `${size}px`
      span.style.height = `${size}px`
      span.style.left = `${event.clientX - rect.left - size / 2}px`
      span.style.top = `${event.clientY - rect.top - size / 2}px`
      span.style.background = binding.value || 'rgb(var(--accent) / 0.25)'
      span.style.animation = 'ripple 0.6s linear'

      el.appendChild(span)
      span.addEventListener('animationend', () => span.remove())
    }

    el.addEventListener('click', el.__rippleHandler__)
  },
  unmounted(el: RippleEl) {
    if (el.__rippleHandler__) {
      el.removeEventListener('click', el.__rippleHandler__)
    }
  }
}
