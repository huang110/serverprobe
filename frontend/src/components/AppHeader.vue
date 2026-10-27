<script setup lang="ts">
/**
 * 顶部导航栏：品牌 Logo、页面菜单、主题切换、偏好设置、退出。
 * 小屏幕下菜单与设置收纳。
 */
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/websocket'
import { useToastStore } from '@/stores/toast'
import { usePrefsStore } from '@/stores/prefs'
import AppLogo from '@/components/AppLogo.vue'
import type { ContrastMode, DensityMode, ThemeMode } from '@/stores/prefs'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const wsStore = useWebSocketStore()
const toastStore = useToastStore()
const prefsStore = usePrefsStore()

const allNavItems = [
  { name: 'agent-list', label: '探针列表', to: '/agents', icon: 'M4 6h16M4 12h16M4 18h16' },
  { name: 'alert-rules', label: '告警规则', to: '/alert/rules', icon: 'M12 6v6l4 2' },
  { name: 'alert-records', label: '告警记录', to: '/alert/records', icon: 'M4 6h16M4 12h10M4 18h7' }
]

// 游客态仅展示公开的探针列表；登录后展示全部导航
const navItems = computed(() =>
  authStore.isAuthenticated
    ? allNavItems
    : allNavItems.filter((item) => item.name === 'agent-list')
)

// ---------- 偏好设置弹层 ----------
const settingsOpen = ref(false)
const settingsRef = ref<HTMLElement | null>(null)

function toggleSettings() {
  settingsOpen.value = !settingsOpen.value
}

function closeSettings(e: MouseEvent) {
  if (settingsRef.value && !settingsRef.value.contains(e.target as Node)) {
    settingsOpen.value = false
  }
}
onMounted(() => document.addEventListener('click', closeSettings))
onBeforeUnmount(() => document.removeEventListener('click', closeSettings))

const themeOptions: { value: ThemeMode; label: string; icon: string }[] = [
  { value: 'dark', label: '深色', icon: 'M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z' },
  { value: 'light', label: '浅色', icon: 'M12 4V2m0 20v-2m8-8h2M2 12h2m13.66-5.66l1.41-1.41M4.93 19.07l1.41-1.41m0-11.32L4.93 4.93m14.14 14.14l-1.41-1.41M12 8a4 4 0 100 8 4 4 0 000-8z' },
  { value: 'auto', label: '跟随系统', icon: 'M3 5h18v11H3z M8 20h8 M12 16v4' }
]

const metricOptions = [
  { key: 'cpu', label: 'CPU' },
  { key: 'memory', label: '内存' },
  { key: 'load', label: '负载' },
  { key: 'network', label: '网络' },
  { key: 'latency', label: 'TCP 延迟' }
]

const refreshOptions = [1, 5, 10]
const densityOptions: { value: DensityMode; label: string }[] = [
  { value: 'compact', label: '紧凑' },
  { value: 'comfortable', label: '舒适' },
  { value: 'spacious', label: '宽松' }
]
const contrastChoices: { v: ContrastMode; l: string }[] = [
  { v: 'normal', l: '标准' },
  { v: 'high', l: '高对比度' }
]

// 快速主题切换（点击图标在深/浅间切换；通过设置选择 auto）
function quickToggleTheme() {
  prefsStore.theme = prefsStore.isDark ? 'light' : 'dark'
  toastStore.info(prefsStore.isDark ? '已切换浅色主题' : '已切换深色主题')
}

function handleLogout() {
  authStore.logout()
  // 退出后降级为游客连接（公开大屏仍可看实时数据），不断开
  wsStore.reconnectWithAuth()
  toastStore.success('已退出登录')
  router.push('/agents')
}

/** 游客点击「管理员登录」 */
function goLogin() {
  router.push('/login')
}
</script>

<template>
  <header class="sticky top-0 z-50 border-b border-line bg-bg/85 backdrop-blur-md">
    <div class="mx-auto flex h-14 max-w-[1600px] items-center gap-3 px-4 sm:gap-4 sm:px-6">
      <!-- Logo -->
      <router-link to="/agents" aria-label="ServerProbe 首页">
        <AppLogo :size="32" />
        <span class="sr-only">ServerProbe</span>
      </router-link>

      <!-- 桌面导航 -->
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
              : 'text-muted hover:bg-surface-2 hover:text-content'
          "
        >{{ item.label }}</router-link>
      </nav>

      <div class="ml-auto flex items-center gap-2 sm:gap-3">
        <!-- 连接状态 -->
        <span class="hidden items-center gap-1.5 text-xs text-muted sm:flex">
          <span
            class="h-2 w-2 rounded-full"
            :class="wsStore.connected ? 'bg-success' : 'animate-pulse bg-warning'"
          ></span>
          {{ wsStore.connectionLabel }}
        </span>

        <!-- 主题切换 -->
        <button
          aria-label="切换深色/浅色主题"
          class="rounded-lg p-2 text-muted transition hover:bg-surface-2 hover:text-content"
          @click="quickToggleTheme"
        >
          <svg v-if="prefsStore.isDark" class="h-[18px] w-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <circle cx="12" cy="12" r="4" />
            <path stroke-linecap="round" d="M12 2v2m0 16v2M4.9 4.9l1.4 1.4m11.4 11.4l1.4 1.4M2 12h2m16 0h2M4.9 19.1l1.4-1.4M17.7 6.3l1.4-1.4" />
          </svg>
          <svg v-else class="h-[18px] w-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M21 12.8A9 9 0 1111.2 3 7 7 0 0021 12.8z" />
          </svg>
        </button>

        <!-- 偏好设置 -->
        <div ref="settingsRef" class="relative">
          <button
            aria-label="打开显示偏好设置"
            aria-haspopup="dialog"
            :aria-expanded="settingsOpen"
            class="rounded-lg p-2 text-muted transition hover:bg-surface-2 hover:text-content"
            @click="toggleSettings"
          >
            <svg class="h-[18px] w-[18px]" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M10.34 5.3a2 2 0 013.32 0l.4.53 1.7-.2a2 2 0 012.2 2.2l-.2 1.7.53.4a2 2 0 010 3.32l-.53.4.2 1.7a2 2 0 01-2.2 2.2l-1.7-.2-.4.53a2 2 0 01-3.32 0l-.4-.53-1.7.2a2 2 0 01-2.2-2.2l.2-1.7-.53-.4a2 2 0 010-3.32l.53-.4-.2-1.7a2 2 0 012.2-2.2l1.7.2z" />
              <circle cx="12" cy="12" r="3" />
            </svg>
          </button>

          <!-- 设置面板 -->
          <Transition name="drop">
            <div
              v-if="settingsOpen"
              role="dialog"
              aria-label="显示偏好设置"
              class="absolute right-0 top-12 z-50 w-72 rounded-2xl border border-line bg-surface p-4 shadow-2xl shadow-black/40"
            >
              <!-- 主题 -->
              <p class="mb-2 text-xs font-semibold text-muted">主题模式</p>
              <div class="mb-4 grid grid-cols-3 gap-1.5">
                <button
                  v-for="t in themeOptions"
                  :key="t.value"
                  class="flex flex-col items-center gap-1 rounded-lg border px-2 py-2 text-[11px] transition"
                  :class="prefsStore.theme === t.value ? 'border-accent bg-accent/10 text-accent' : 'border-line text-muted hover:text-content'"
                  @click="prefsStore.theme = t.value"
                >
                  <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                    <path stroke-linecap="round" stroke-linejoin="round" :d="t.icon" />
                  </svg>
                  {{ t.label }}
                </button>
              </div>

              <!-- 对比度 -->
              <p class="mb-2 text-xs font-semibold text-muted">对比度</p>
              <div class="mb-4 grid grid-cols-2 gap-1.5">
                <button
                  v-for="c in contrastChoices"
                  :key="c.v"
                  class="rounded-lg border px-2 py-1.5 text-[11px] transition"
                  :class="prefsStore.contrast === c.v ? 'border-accent bg-accent/10 text-accent' : 'border-line text-muted hover:text-content'"
                  @click="prefsStore.contrast = c.v"
                >{{ c.l }}</button>
              </div>

              <!-- 密度 -->
              <p class="mb-2 text-xs font-semibold text-muted">界面密度</p>
              <div class="mb-4 grid grid-cols-3 gap-1.5">
                <button
                  v-for="d in densityOptions"
                  :key="d.value"
                  class="rounded-lg border px-2 py-1.5 text-[11px] transition"
                  :class="prefsStore.density === d.value ? 'border-accent bg-accent/10 text-accent' : 'border-line text-muted hover:text-content'"
                  @click="prefsStore.density = d.value"
                >{{ d.label }}</button>
              </div>

              <!-- 刷新频率 -->
              <p class="mb-2 text-xs font-semibold text-muted">数据刷新频率</p>
              <div class="mb-4 grid grid-cols-3 gap-1.5">
                <button
                  v-for="r in refreshOptions"
                  :key="r"
                  class="rounded-lg border px-2 py-1.5 text-[11px] transition"
                  :class="prefsStore.refreshInterval === r ? 'border-accent bg-accent/10 text-accent' : 'border-line text-muted hover:text-content'"
                  @click="prefsStore.refreshInterval = r"
                >{{ r }}s</button>
              </div>

              <!-- 卡片指标 -->
              <p class="mb-2 text-xs font-semibold text-muted">卡片显示指标</p>
              <div class="grid grid-cols-2 gap-1.5">
                <button
                  v-for="opt in metricOptions"
                  :key="opt.key"
                  class="flex items-center justify-between rounded-lg border px-2.5 py-1.5 text-[11px] transition"
                  :class="prefsStore.cardMetrics.includes(opt.key) ? 'border-success/60 text-success' : 'border-line text-muted'"
                  @click="prefsStore.toggleCardMetric(opt.key)"
                >
                  {{ opt.label }}
                  <span>{{ prefsStore.cardMetrics.includes(opt.key) ? '✓' : '＋' }}</span>
                </button>
              </div>
            </div>
          </Transition>
        </div>

        <!-- 游客：管理员登录入口（高亮） -->
        <button
          v-if="!authStore.isAuthenticated"
          class="btn-primary px-3 py-1.5 text-xs"
          aria-label="管理员登录"
          @click="goLogin"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15m3 0l3-3m0 0l-3-3m3 3H9" />
          </svg>
          <span class="hidden sm:inline">管理员登录</span>
        </button>

        <!-- 已登录：退出 -->
        <button
          v-else
          aria-label="退出登录"
          class="btn-ghost px-3 py-1.5 text-xs"
          @click="handleLogout"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 9V5.25A2.25 2.25 0 0013.5 3h-6a2.25 2.25 0 00-2.25 2.25v13.5A2.25 2.25 0 007.5 21h6a2.25 2.25 0 002.25-2.25V15M12 9l-3 3m0 0l3 3m-3-3h12.75" />
          </svg>
          <span class="hidden sm:inline">退出</span>
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
        :class="route.name === item.name ? 'bg-accent/15 text-accent' : 'text-muted hover:bg-surface-2'"
      >{{ item.label }}</router-link>
    </nav>
  </header>
</template>

<style scoped>
.drop-enter-active {
  transition: all 0.18s ease;
  transform-origin: top right;
}
.drop-enter-from {
  opacity: 0;
  transform: translateY(-6px) scale(0.97);
}
</style>
