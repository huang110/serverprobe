<script setup lang="ts">
/**
 * 可排序表头单元格。
 * 点击在 升序 -> 降序 -> 不排序 间循环，显示排序箭头。
 */
import { computed } from 'vue'

type SortDir = 'asc' | 'desc' | null

const props = defineProps<{
  /** 当前列字段名 */
  field: string
  /** 当前排序字段 */
  sortField?: string
  /** 当前排序方向 */
  sortDir?: SortDir
  /** 是否支持排序 */
  sortable?: boolean
  align?: 'left' | 'right' | 'center'
}>()

const emit = defineEmits<{
  sort: [payload: { field: string; dir: SortDir }]
}>()

const active = computed(() => props.sortField === props.field && props.sortDir)

function toggle() {
  if (!props.sortable) return
  if (props.sortField !== props.field) emit('sort', { field: props.field, dir: 'asc' })
  else if (props.sortDir === 'asc') emit('sort', { field: props.field, dir: 'desc' })
  else emit('sort', { field: props.field, dir: null })
}
</script>

<template>
  <th
    :class="[
      sortable && 'sortable-th',
      align === 'right' && 'text-right',
      align === 'center' && 'text-center'
    ]"
    :aria-sort="
      active === 'asc' ? 'ascending' : active === 'desc' ? 'descending' : undefined
    "
    @click="toggle"
  >
    <span class="inline-flex items-center gap-1">
      <slot />
      <svg
        v-if="sortable"
        class="h-3 w-3 transition-transform"
        :class="[
          active ? 'text-accent' : 'text-line',
          active === 'desc' && 'rotate-180'
        ]"
        viewBox="0 0 12 12"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M6 2 L10 7 H2 Z" />
      </svg>
    </span>
  </th>
</template>
