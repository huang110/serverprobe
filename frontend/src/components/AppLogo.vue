<script setup lang="ts">
/**
 * ServerProbe 品牌标识：
 * 以「监控探头 / 心跳波形」为概念的独特几何 Mark，
 * 深色底板 + 渐变波形，区别于通用服务器机柜图标。
 */
withDefaults(defineProps<{ size?: number; showText?: boolean }>(), {
  size: 32,
  showText: false
})
</script>

<template>
  <span class="inline-flex items-center gap-2">
    <svg
      :width="size"
      :height="size"
      viewBox="0 0 40 40"
      fill="none"
      role="img"
      aria-label="ServerProbe logo"
    >
      <defs>
        <linearGradient id="sp-logo-grad" x1="0" y1="0" x2="40" y2="40" gradientUnits="userSpaceOnUse">
          <stop stop-color="#38bdf8" />
          <stop offset="0.55" stop-color="#3b82f6" />
          <stop offset="1" stop-color="#8b5cf6" />
        </linearGradient>
      </defs>
      <!-- 圆角底板 -->
      <rect x="1" y="1" width="38" height="38" rx="11" fill="rgb(var(--surface2))" stroke="url(#sp-logo-grad)" stroke-width="1.6" />
      <!-- 探头心跳波形 -->
      <path
        d="M7 23h5l3-8 5 16 4-11 2.5 3H33"
        stroke="url(#sp-logo-grad)"
        stroke-width="2.4"
        stroke-linecap="round"
        stroke-linejoin="round"
      />
      <!-- 探测点 -->
      <circle cx="33" cy="23" r="2.6" fill="#38bdf8" />
      <circle cx="33" cy="23" r="4.4" stroke="#38bdf8" stroke-opacity="0.35" stroke-width="1.2" />
    </svg>
    <span v-if="showText" class="text-base font-bold tracking-tight text-content">ServerProbe</span>
  </span>
</template>
