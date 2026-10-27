/**
 * IP 地址打码工具（晒机截图隐私保护）。
 * 后端对游客也会返回掩码 IP（如 36.232.*.*）；
 * 前端「隐藏 IP」开关在此基础上进一步模糊，或对已登录可见的完整 IP 打码。
 */

/** 将 IPv4 后两段打星号：1.2.3.4 -> 1.2.*.* */
export function maskIp(ip: string): string {
  if (!ip) return '-'
  const v4 = ip.trim().match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/)
  if (v4) {
    return `${v4[1]}.${v4[2]}.*.*`
  }
  // IPv6：保留前两段，其余省略
  if (ip.includes(':')) {
    const parts = ip.split(':')
    if (parts.length > 2) return `${parts[0]}:${parts[1]}:****`
    return ip
  }
  // 主机名或其他：保留前两位字符
  return ip.length > 2 ? ip.slice(0, 2) + '***' : ip
}

/** 根据开关决定显示原始 IP 还是打码 IP */
export function displayIp(ip: string, hidden: boolean): string {
  if (!ip) return '-'
  return hidden ? maskIp(ip) : ip
}
