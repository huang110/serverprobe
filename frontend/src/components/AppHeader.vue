<script setup lang="ts">
/** 顶部导航栏：Logo、页面菜单、WebSocket 状态、退出登录 */
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/websocket'
import { useToastStore } from '@/stores/toast'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const wsStore = useWebSocketStore()
const toastStore = useToastStore()

const navItems = [
  { name: 'agent-list', label: 'Agent 列表', to: '/agents' },
  { name: 'alert-rules', label: '告警规则', to: '/alert/rules' },
  { name: 'alert-records', label: '告警记录', to: '/alert/records' }
]

function handleLogout() {
  wsStore.disconnect()
  authStore.logout()
  toastStore.success('已退出登录')
  router.push('/login')
}
</script>

<template>
  <header class="sticky top-0 z-50 border-b border-line bg-ink-900/85 backdrop-blur-md">
    <div class="mx-auto flex h-14 max-w-[1600px] items-center gap-4 px-4 sm:px-6">
      <!-- Logo -->
      <router-link to="/agents" class="flex shrink-0 items-center gap-2">
        <span class="flex h-8 w-8 items-center justify-center rounded-lg bg-accent/15">
          <svg class="h-5 w-5 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <rect x="3" y="4" width="18" height="6" rx="1.5" />
            <rect x="3" y="12" width="18" height="4" rx="1.5" />
            <rect x="3" y="18" width="18" height="3" rx="1.5" />
          </svg>
        </span>
        <span class="hidden text-base font-bold tracking-tight text-white sm:block">
          ServerProbe
        </span>
      </router-link>

      <!-- 导航菜单 -->
      <nav class="hidden items-center gap-1 md:flex">
        <router-link
          v-for="item in navItems"
          :key="item.name"
          :to="item.to"
          class="rounded-lg px-3 py-1.5 text-sm font-medium transition"
          :class="
            route.name === item.name ||
            (item.name === 'agent-list' && route.name === 'agent-detail')
              ? 'bg-accent/15 text-accent'
              : 'text-muted hover:bg-ink-700 hover:text-slate-200'
          "
        >
          {{ item.label }}
        </router-link>
      </nav>

      <div class="ml-auto flex items-center gap-3">
        <!-- WebSocket 连接状态 -->
        <span class="hidden items-center gap-1.5 text-xs text-muted sm:flex">
          <span
            class="h-2 w-2 rounded-full"
            :class="wsStore.connected ? 'bg-success' : 'bg-warning animate-pulse'"
          ></span>
          {{ wsStore.connected ? '实时' : '连接中' }}
        </span>

        <button class="btn-ghost px-3 py-1.5 text-xs" @click="handleLogout">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15M12 9l-3 3m0 0l3 3m-3-3h12.75" />
          </svg>
          退出
        </button>
      </div>
    </div>

    <!-- 移动端导航 -->
    <nav class="flex items-center gap-1 overflow-x-auto border-t border-line px-3 py-2 md:hidden">
      <router-link
        v-for="item in navItems"
        :key="item.name"
        :to="item.to"
        class="whitespace-nowrap rounded-lg px-3 py-1 text-xs font-medium transition"
        :class="
          route.name === item.name
            ? 'bg-accent/15 text-accent'
            : 'text-muted hover:bg-ink-700'
        "
      >
        {{ item.label }}
      </router-link>
    </nav>
  </header>
</template>
