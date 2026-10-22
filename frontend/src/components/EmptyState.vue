<script setup lang="ts">
/** 空状态占位：插画图标 + 文案，支持多种语义变体 */
import { computed } from 'vue'

type Variant = 'default' | 'agent' | 'chart' | 'alert' | 'search' | 'error'

const props = withDefaults(
  defineProps<{
    text?: string
    hint?: string
    variant?: Variant
  }>(),
  {
    text: '暂无数据',
    hint: '',
    variant: 'default'
  }
)

// 各变体使用不同图标与强调色
const accent = computed(() => {
  switch (props.variant) {
    case 'alert':
      return 'rgb(var(--warning))'
    case 'error':
      return 'rgb(var(--danger))'
    case 'search':
      return 'rgb(var(--info))'
    default:
      return 'rgb(var(--accent))'
  }
})
</script>

<template>
  <div class="flex flex-col items-center justify-center gap-3 py-14 text-center">
    <div class="relative">
      <!-- 背景光晕 -->
      <div
        class="absolute inset-0 -z-10 blur-2xl"
        :style="{ background: `radial-gradient(circle, ${accent}22, transparent 70%)` }"
      ></div>

      <!-- 变体插画 -->
      <svg
        v-if="variant === 'agent'"
        class="h-16 w-16"
        viewBox="0 0 64 64"
        fill="none"
        aria-hidden="true"
      >
        <rect x="10" y="16" width="44" height="14" rx="4" :stroke="accent" stroke-width="2.5" opacity="0.9" />
        <rect x="10" y="34" width="44" height="14" rx="4" :stroke="accent" stroke-width="2.5" opacity="0.55" />
        <circle cx="17" cy="23" r="2" :fill="accent" />
        <circle cx="17" cy="41" r="2" fill="rgb(var(--success))" />
        <path d="M26 23h20M26 41h14" :stroke="accent" stroke-width="2.5" stroke-linecap="round" opacity="0.4" />
      </svg>

      <svg v-else-if="variant === 'chart'" class="h-16 w-16" viewBox="0 0 64 64" fill="none" aria-hidden="true">
        <path d="M12 46 L24 34 L32 40 L52 20" :stroke="accent" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" />
        <path d="M40 20h12v12" :stroke="accent" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.6" />
        <line x1="10" y1="52" x2="54" y2="52" :stroke="accent" stroke-width="2.5" stroke-linecap="round" opacity="0.3" />
      </svg>

      <svg v-else-if="variant === 'alert'" class="h-16 w-16" viewBox="0 0 64 64" fill="none" aria-hidden="true">
        <path
          d="M32 10 L56 52 H8 Z"
          :stroke="accent"
          stroke-width="2.5"
          stroke-linejoin="round"
          opacity="0.9"
        />
        <line x1="32" y1="26" x2="32" y2="38" :stroke="accent" stroke-width="3" stroke-linecap="round" />
        <circle cx="32" cy="45" r="2" :fill="accent" />
      </svg>

      <svg v-else-if="variant === 'search'" class="h-16 w-16" viewBox="0 0 64 64" fill="none" aria-hidden="true">
        <circle cx="28" cy="28" r="16" :stroke="accent" stroke-width="2.5" opacity="0.9" />
        <path d="M40 40 L52 52" :stroke="accent" stroke-width="3" stroke-linecap="round" />
        <path d="M20 28h16M28 20v16" :stroke="accent" stroke-width="2" stroke-linecap="round" opacity="0.35" />
      </svg>

      <svg v-else class="h-16 w-16" viewBox="0 0 64 64" fill="none" aria-hidden="true">
        <rect x="8" y="18" width="48" height="34" rx="6" :stroke="accent" stroke-width="2.5" opacity="0.85" />
        <path d="M8 28h48" :stroke="accent" stroke-width="2.5" opacity="0.4" />
        <circle cx="16" cy="23" r="1.6" :fill="accent" />
        <circle cx="22" cy="23" r="1.6" :fill="accent" />
        <path d="M20 38h24M20 44h14" :stroke="accent" stroke-width="2.5" stroke-linecap="round" opacity="0.35" />
      </svg>
    </div>

    <p class="text-sm font-medium text-content">{{ text }}</p>
    <p v-if="hint" class="max-w-xs text-xs leading-relaxed text-muted">{{ hint }}</p>
    <div v-if="$slots.default" class="mt-1">
      <slot />
    </div>
  </div>
</template>
