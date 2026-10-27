/**
 * Axios 实例封装。
 * - 请求拦截器自动携带 JWT Token
 * - 响应拦截器统一解包 data，并处理 401（Token 失效）
 */
import axios, { AxiosError } from 'axios'

const apiClient = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '',
  timeout: 10000,
  headers: { 'Content-Type': 'application/json' }
})

// 请求拦截器：添加 JWT Token
apiClient.interceptors.request.use((config) => {
  const token = localStorage.getItem('token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

// 响应拦截器：统一处理错误
apiClient.interceptors.response.use(
  (response) => response,
  (error: AxiosError<{ success?: boolean; message?: string }>) => {
    const status = error.response?.status

    if (status === 401) {
      const path = window.location.pathname
      // 公开探针页面（/agents、/agents/:id）允许游客访问，401 时不强制踢去登录页
      const isPublicAgentPage = /^\/agents(\/|$)/.test(path)
      if (!isPublicAgentPage) {
        // Token 过期或非法：清理登录态并跳转登录页。
        // 此处处于 setup 之外，无法使用 useRouter，直接通过 localStorage + 跳转处理。
        localStorage.removeItem('token')
        if (path !== '/login') {
          window.location.href = '/login'
        }
      }
    }

    // 提取后端返回的错误信息，供调用方展示
    const message = error.response?.data?.message || error.message || '请求失败'
    return Promise.reject(new Error(message))
  }
)

export default apiClient
