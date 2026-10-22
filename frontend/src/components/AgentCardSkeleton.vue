<script setup lang="ts">
/** Agent 卡片骨架屏 */
withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })
</script>

<template>
  <div class="card h-full overflow-hidden p-4" data-testid="agent-card-skeleton">
    <div class="mb-3 flex items-start justify-between gap-2">
      <div class="flex-1 space-y-2">
        <div class="skeleton h-4 w-2/3"></div>
        <div class="skeleton h-3 w-1/3"></div>
      </div>
      <div class="skeleton h-5 w-14 rounded-full"></div>
    </div>

    <template v-if="!compact">
      <div class="space-y-3">
        <div v-for="i in 3" :key="i">
          <div class="mb-1 flex justify-between">
            <div class="skeleton h-3 w-10"></div>
            <div class="skeleton h-3 w-12"></div>
          </div>
          <div class="skeleton h-1.5 w-full"></div>
        </div>
        <div class="flex justify-between border-t border-line/60 pt-2">
          <div class="skeleton h-3 w-16"></div>
          <div class="skeleton h-3 w-16"></div>
        </div>
      </div>
    </template>
  </div>
</template>
