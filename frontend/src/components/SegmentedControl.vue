<script setup lang="ts" generic="T extends string">
/**
 * 分段控制器：带滑动高亮指示条。
 * 高亮块根据当前选中项的位置与宽度过渡滑动。
 */
import { computed, nextTick, onMounted, ref, watch } from 'vue'

interface Option {
  value: T
  label: string
  icon?: string
}

const props = defineProps<{
  modelValue: T
  options: Option[]
  size?: 'sm' | 'md'
  ariaLabel?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: T] }>()

const containerRef = ref<HTMLElement | null>(null)
const optionEls = ref<(HTMLElement | null)[]>([])

const indicatorLeft = ref(0)
const indicatorWidth = ref(0)
const indicatorReady = ref(false)

function measure() {
  const idx = props.options.findIndex((o) => o.value === props.modelValue)
  const el = optionEls.value[idx]
  if (!el) return
  indicatorLeft.value = el.offsetLeft
  indicatorWidth.value = el.offsetWidth
  indicatorReady.value = true
}

onMounted(() => nextTick(measure))
watch(() => props.modelValue, () => nextTick(measure))
watch(() => props.options, () => nextTick(measure))

const sizeClass = computed(() =>
  props.size === 'sm' ? 'px-2.5 py-1 text-xs' : 'px-3 py-1.5 text-xs'
)
</script>

<template>
  <div
    ref="containerRef"
    role="tablist"
    :aria-label="ariaLabel || 'view switcher'"
    class="relative inline-flex rounded-lg border border-line bg-surface-2 p-0.5"
  >
    <!-- 滑动指示条 -->
    <span
      class="absolute bottom-0.5 top-0.5 rounded-md bg-accent transition-all duration-300 ease-out"
      :class="!indicatorReady && 'opacity-0'"
      :style="{ left: indicatorLeft + 'px', width: indicatorWidth + 'px' }"
    ></span>

    <button
      v-for="(opt, i) in options"
      :key="opt.value"
      :ref="(el) => (optionEls[i] = el as HTMLElement | null)"
      type="button"
      role="tab"
      :aria-selected="modelValue === opt.value"
      class="relative z-10 inline-flex items-center gap-1 rounded-md font-medium transition-colors duration-200"
      :class="[sizeClass, modelValue === opt.value ? 'text-white' : 'text-muted hover:text-content']"
      @click="emit('update:modelValue', opt.value)"
    >
      <span v-if="opt.icon" v-html="opt.icon"></span>
      {{ opt.label }}
    </button>
  </div>
</template>
