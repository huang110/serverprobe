<script setup lang="ts">
/** 客户端分页器 */
import { computed } from 'vue'

const props = defineProps<{
  page: number
  pageSize: number
  total: number
  pageSizes?: number[]
}>()

const emit = defineEmits<{
  'update:page': [value: number]
  'update:pageSize': [value: number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

// 最多展示 7 个页码，超出部分以省略号表示
const pageItems = computed<(number | '...')[]>(() => {
  const total = totalPages.value
  const current = props.page
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1)

  const items: (number | '...')[] = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(total - 1, current + 1)

  if (start > 2) items.push('...')
  for (let i = start; i <= end; i++) items.push(i)
  if (end < total - 1) items.push('...')
  items.push(total)
  return items
})

function go(target: number) {
  if (target < 1 || target > totalPages.value || target === props.page) return
  emit('update:page', target)
}
</script>

<template>
  <div class="flex flex-wrap items-center justify-between gap-3 px-1 py-3 text-sm">
    <div class="flex items-center gap-2 text-muted">
      <span>共 <span class="font-medium text-slate-300">{{ total }}</span> 条</span>
      <select
        class="form-input w-auto py-1 text-xs"
        :value="pageSize"
        @change="emit('update:pageSize', Number(($event.target as HTMLSelectElement).value))"
      >
        <option v-for="size in pageSizes || [20, 50, 100]" :key="size" :value="size">
          {{ size }} 条/页
        </option>
      </select>
    </div>

    <div class="flex items-center gap-1">
      <button class="btn-ghost px-2 py-1 text-xs" :disabled="page === 1" @click="go(page - 1)">
        上一页
      </button>
      <template v-for="(item, idx) in pageItems" :key="idx">
        <span v-if="item === '...'" class="px-1 text-muted">…</span>
        <button
          v-else
          class="h-8 min-w-8 rounded-lg px-2 text-xs transition"
          :class="
            item === page
              ? 'bg-accent font-semibold text-white'
              : 'border border-line text-slate-300 hover:border-slate-500'
          "
          @click="go(item)"
        >
          {{ item }}
        </button>
      </template>
      <button
        class="btn-ghost px-2 py-1 text-xs"
        :disabled="page === totalPages"
        @click="go(page + 1)"
      >
        下一页
      </button>
    </div>
  </div>
</template>
