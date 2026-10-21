/**
 * 格式化工具函数。
 */

/**
 * 字节数格式化为易读字符串。
 * @param bytes 字节数
 * @param decimals 保留小数位
 */
export function formatBytes(bytes: number, decimals = 1): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(k)), sizes.length - 1)
  return `${(bytes / Math.pow(k, i)).toFixed(i === 0 ? 0 : decimals)} ${sizes[i]}`
}

/** 字节速率格式化为「xx MB/s」形式 */
export function formatRate(bytesPerSecond: number, decimals = 2): string {
  if (!Number.isFinite(bytesPerSecond) || bytesPerSecond <= 0) return '0 B/s'
  const k = 1024
  const sizes = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s']
  const i = Math.min(Math.floor(Math.log(bytesPerSecond) / Math.log(k)), sizes.length - 1)
  return `${(bytesPerSecond / Math.pow(k, i)).toFixed(i === 0 ? 0 : decimals)} ${sizes[i]}`
}

/** 秒级时间戳格式化为完整日期时间字符串 */
export function formatTimestamp(timestamp: number): string {
  if (!timestamp) return '-'
  return new Date(timestamp * 1000).toLocaleString('zh-CN', { hour12: false })
}

/** ISO 字符串或秒级时间戳统一转「YYYY-MM-DD HH:mm:ss」 */
export function formatDateTime(value: number | string): string {
  if (!value) return '-'
  const date = typeof value === 'string' ? new Date(value) : new Date(value * 1000)
  if (Number.isNaN(date.getTime())) return '-'
  return date.toLocaleString('zh-CN', { hour12: false })
}

/**
 * 相对时间（如「5 秒前」）。
 * @param timestamp 秒级时间戳
 */
export function formatRelativeTime(timestamp: number): string {
  if (!timestamp) return '-'
  const now = Date.now() / 1000
  const diff = Math.max(0, now - timestamp)

  if (diff < 60) return `${Math.floor(diff)} 秒前`
  if (diff < 3600) return `${Math.floor(diff / 60)} 分钟前`
  if (diff < 86400) return `${Math.floor(diff / 3600)} 小时前`
  if (diff < 86400 * 30) return `${Math.floor(diff / 86400)} 天前`
  return formatTimestamp(timestamp)
}

/**
 * 告警持续时长。
 * @param firedAt 触发时间（秒）
 * @param resolvedAt 恢复时间（秒），0 表示仍在触发
 */
export function formatDuration(firedAt: number, resolvedAt: number): string {
  if (!firedAt) return '-'
  const end = resolvedAt || Date.now() / 1000
  const seconds = Math.max(0, Math.floor(end - firedAt))

  const days = Math.floor(seconds / 86400)
  const hours = Math.floor((seconds % 86400) / 3600)
  const minutes = Math.floor((seconds % 3600) / 60)

  if (days > 0) return `${days}天${hours}小时`
  if (hours > 0) return `${hours}小时${minutes}分钟`
  if (minutes > 0) return `${minutes}分钟`
  return `${seconds}秒`
}

/** 秒数转「x 分钟 / x 小时」 */
export function formatSeconds(seconds: number): string {
  if (!seconds) return '-'
  if (seconds < 60) return `${seconds} 秒`
  if (seconds < 3600) return `${Math.round(seconds / 60)} 分钟`
  if (seconds < 86400) return `${(seconds / 3600).toFixed(1)} 小时`
  return `${(seconds / 86400).toFixed(1)} 天`
}

/** 百分比文本，保留一位小数 */
export function formatPercent(value: number): string {
  if (!Number.isFinite(value)) return '--'
  return `${value.toFixed(1)}%`
}

/** 根据使用率返回对应语义颜色 */
export function usageColor(percent: number): string {
  if (percent >= 90) return '#f43f5e'
  if (percent >= 75) return '#f5a524'
  return '#22c55e'
}
