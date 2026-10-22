<script setup lang="ts">
/**
 * 应用根组件：
 * - 已登录页面展示统一的顶部导航
 * - 全局挂载通知容器
 * - 注册全局快捷键
 */
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import ToastContainer from '@/components/ToastContainer.vue'
import { useShortcuts } from '@/composables/useShortcuts'

const route = useRoute()
const showChrome = computed(() => route.meta.public !== true)

useShortcuts()
</script>

<template>
  <div class="min-h-full">
    <AppHeader v-if="showChrome" />
    <main :class="showChrome ? 'mx-auto max-w-[1600px] px-4 py-6 sm:px-6' : ''">
      <router-view v-slot="{ Component }">
        <transition name="page" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
    <ToastContainer />
  </div>
</template>

<style scoped>
.page-enter-active {
  transition: opacity 0.18s ease;
}
.page-enter-from {
  opacity: 0;
}
</style>
