/**
 * WebSocket 客户端封装：
 * - 支持按消息类型订阅
 * - 断线自动重连（固定 5 秒间隔）
 * - 自动心跳，保持连接
 *
 * 连接地址根据环境变量决定：
 *  - VITE_WS_BASE_URL 有值时使用 `${base}/ws?token=xxx`
 *  - 为空时使用相对路径（生产环境同源 / 开发环境走 Vite 代理）。
 */

type MessageHandler = (data: any) => void

const RECONNECT_DELAY = 5000
const HEARTBEAT_INTERVAL = 25000

export class WebSocketClient {
  private ws: WebSocket | null = null
  private url = ''
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private heartbeatTimer: ReturnType<typeof setInterval> | null = null
  private manuallyClosed = false
  private callbacks = new Map<string, Set<MessageHandler>>()

  /** 连接状态变化回调 */
  public onStatusChange: ((connected: boolean) => void) | null = null

  /** 建立连接；token 为空时以游客身份连接 /ws（后端允许匿名订阅） */
  connect(token: string): void {
    this.manuallyClosed = false
    this.url = this.buildUrl(token)

    this.createSocket()
  }

  private createSocket(): void {
    let socket: WebSocket
    try {
      socket = new WebSocket(this.url)
    } catch (err) {
      console.error('[ws] 创建连接失败:', err)
      this.scheduleReconnect()
      return
    }
    this.ws = socket

    socket.onopen = () => {
      console.log('[ws] connected')
      this.clearReconnectTimer()
      this.startHeartbeat()
      this.onStatusChange?.(true)
    }

    socket.onmessage = (event) => {
      this.dispatch(event.data)
    }

    socket.onclose = () => {
      console.log('[ws] closed')
      this.stopHeartbeat()
      this.onStatusChange?.(false)
      if (!this.manuallyClosed) this.scheduleReconnect()
    }

    socket.onerror = (error) => {
      console.error('[ws] error:', error)
      // 关闭动作会紧随其后触发 onclose，在此不必重复处理
    }
  }

  /** 订阅指定类型消息，返回取消订阅函数 */
  on(type: string, handler: MessageHandler): () => void {
    let handlers = this.callbacks.get(type)
    if (!handlers) {
      handlers = new Set()
      this.callbacks.set(type, handlers)
    }
    handlers.add(handler)
    return () => handlers!.delete(handler)
  }

  /** 发送消息 */
  send(payload: unknown): boolean {
    if (!this.ws || this.ws.readyState !== WebSocket.OPEN) return false
    this.ws.send(typeof payload === 'string' ? payload : JSON.stringify(payload))
    return true
  }

  /** 主动断开，不再重连 */
  disconnect(): void {
    this.manuallyClosed = true
    this.clearReconnectTimer()
    this.stopHeartbeat()
    if (this.ws) {
      this.ws.onclose = null
      this.ws.close()
      this.ws = null
    }
    this.onStatusChange?.(false)
  }

  /**
   * 用新的 token 重新建立连接（登录态切换时使用）。
   * 先彻底关闭旧连接（含取消重连定时器），再以新 URL 建连。
   */
  reconnect(token: string): void {
    this.manuallyClosed = true
    this.clearReconnectTimer()
    this.stopHeartbeat()
    if (this.ws) {
      this.ws.onclose = null
      try {
        this.ws.close()
      } catch {
        // ignore
      }
      this.ws = null
    }
    this.connect(token)
  }

  private dispatch(raw: string): void {
    let message: { type?: string; data?: unknown }
    try {
      message = JSON.parse(raw)
    } catch {
      console.warn('[ws] 无法解析消息:', raw)
      return
    }
    if (!message.type) return

    const handlers = this.callbacks.get(message.type)
    if (handlers) {
      handlers.forEach((cb) => {
        try {
          cb(message.data)
        } catch (err) {
          console.error('[ws] 消息处理异常:', err)
        }
      })
    }
  }

  private startHeartbeat(): void {
    this.stopHeartbeat()
    this.heartbeatTimer = setInterval(() => {
      this.send({ type: 'ping' })
    }, HEARTBEAT_INTERVAL)
  }

  private stopHeartbeat(): void {
    if (this.heartbeatTimer) {
      clearInterval(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private scheduleReconnect(): void {
    this.clearReconnectTimer()
    this.reconnectTimer = setTimeout(() => {
      if (this.manuallyClosed) return
      console.log('[ws] 尝试重连...')
      this.createSocket()
    }, RECONNECT_DELAY)
  }

  private clearReconnectTimer(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
  }

  private buildUrl(token: string): string {
    const base = import.meta.env.VITE_WS_BASE_URL
    const root = base ? base.replace(/\/$/, '') : ''
    // 游客（未登录）：连接 /ws，不带 token
    if (!token) {
      if (base) return `${root}/ws`
      const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
      return `${protocol}//${window.location.host}/ws`
    }
    // 已登录：/ws?token=xxx
    if (base) {
      return `${root}/ws?token=${encodeURIComponent(token)}`
    }
    // 相对地址：生产环境与页面同源；开发环境由 Vite 代理转发到后端
    const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${protocol}//${window.location.host}/ws?token=${encodeURIComponent(token)}`
  }
}

/** 全局单例 */
export const wsClient = new WebSocketClient()
