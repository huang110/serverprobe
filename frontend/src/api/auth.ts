/** 认证相关接口 */
import apiClient from './client'

export interface LoginParams {
  username: string
  password: string
}

export interface LoginResult {
  token: string
}

/** POST /api/auth/login 登录 */
export async function login(params: LoginParams): Promise<string> {
  const { data } = await apiClient.post('/api/auth/login', params)
  if (!data?.success || !data?.data?.token) {
    throw new Error(data?.message || '登录失败')
  }
  return data.data.token as string
}
