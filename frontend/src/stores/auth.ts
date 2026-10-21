/**
 * 认证状态管理。
 * Token 持久化到 localStorage，刷新页面后仍保持登录态。
 */
import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import * as authApi from '@/api/auth'

const TOKEN_KEY = 'token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref<string>(localStorage.getItem(TOKEN_KEY) || '')

  const isAuthenticated = computed(() => Boolean(token.value))

  /** 登录，成功后保存 Token */
  async function login(username: string, password: string): Promise<void> {
    const newToken = await authApi.login({ username, password })
    token.value = newToken
    localStorage.setItem(TOKEN_KEY, newToken)
  }

  /** 退出登录，清除 Token */
  function logout(): void {
    token.value = ''
    localStorage.removeItem(TOKEN_KEY)
  }

  return { token, isAuthenticated, login, logout }
})
