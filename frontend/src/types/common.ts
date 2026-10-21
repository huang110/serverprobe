/** 后端统一响应包裹 */
export interface ApiEnvelope<T> {
  success: boolean
  data?: T
  message?: string
}
