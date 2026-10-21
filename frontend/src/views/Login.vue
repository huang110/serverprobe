<script setup lang="ts">
/**
 * 登录页：居中卡片，调用登录接口并保存 Token。
 */
import { reactive, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useWebSocketStore } from '@/stores/websocket'
import { useToastStore } from '@/stores/toast'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const wsStore = useWebSocketStore()
const toastStore = useToastStore()

const form = reactive({
  username: 'admin',
  password: 'admin123'
})
const loading = ref(false)
const errorMsg = ref('')

async function handleLogin() {
  errorMsg.value = ''

  if (!form.username.trim() || !form.password) {
    errorMsg.value = '请输入用户名和密码'
    return
  }

  loading.value = true
  try {
    await authStore.login(form.username.trim(), form.password)
    toastStore.success('登录成功')

    // 登录成功后建立 WebSocket 连接
    wsStore.connect()

    const redirect = (route.query.redirect as string) || '/agents'
    router.push(redirect)
  } catch (err) {
    errorMsg.value = err instanceof Error ? err.message : '登录失败，请重试'
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="relative flex min-h-screen items-center justify-center overflow-hidden bg-ink-950 px-4">
    <!-- 背景装饰 -->
    <div class="pointer-events-none absolute inset-0">
      <div class="absolute -left-32 -top-32 h-96 w-96 rounded-full bg-accent/10 blur-3xl"></div>
      <div class="absolute -bottom-32 -right-32 h-96 w-96 rounded-full bg-success/10 blur-3xl"></div>
      <div
        class="absolute inset-0 opacity-[0.03]"
        style="background-image: linear-gradient(#fff 1px, transparent 1px), linear-gradient(90deg, #fff 1px, transparent 1px); background-size: 40px 40px;"
      ></div>
    </div>

    <div class="relative z-10 w-full max-w-md animate-fade-in">
      <!-- 标题 -->
      <div class="mb-8 text-center">
        <span class="mx-auto mb-4 flex h-14 w-14 items-center justify-center rounded-2xl bg-accent/15">
          <svg class="h-8 w-8 text-accent" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <rect x="3" y="4" width="18" height="6" rx="1.5" />
            <rect x="3" y="12" width="18" height="4" rx="1.5" />
            <rect x="3" y="18" width="18" height="3" rx="1.5" />
          </svg>
        </span>
        <h1 class="text-2xl font-bold text-white">ServerProbe</h1>
        <p class="mt-1 text-sm text-muted">轻量级服务器监控系统</p>
      </div>

      <!-- 登录卡片 -->
      <form class="card p-7" @submit.prevent="handleLogin">
        <div class="space-y-4">
          <div>
            <label class="form-label">用户名</label>
            <input v-model="form.username" type="text" class="form-input" placeholder="请输入用户名" autocomplete="username" />
          </div>

          <div>
            <label class="form-label">密码</label>
            <input v-model="form.password" type="password" class="form-input" placeholder="请输入密码" autocomplete="current-password" />
          </div>

          <p v-if="errorMsg" class="flex items-center gap-1.5 rounded-lg bg-danger/10 px-3 py-2 text-xs text-danger">
            <svg class="h-4 w-4 shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path stroke-linecap="round" stroke-linejoin="round" d="M12 9v3.75m9-.75a9 9 0 11-18 0 9 9 0 0118 0zm-9 3.75h.008v.008H12v-.008z" />
            </svg>
            {{ errorMsg }}
          </p>

          <button type="submit" class="btn-primary w-full py-2.5" :disabled="loading">
            <span v-if="loading" class="h-4 w-4 animate-spin rounded-full border-2 border-white/40 border-t-white"></span>
            {{ loading ? '登录中...' : '登 录' }}
          </button>
        </div>

        <p class="mt-5 text-center text-xs text-muted">默认账号 admin / admin123</p>
      </form>
    </div>
  </div>
</template>
