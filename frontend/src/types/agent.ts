/**
 * Agent 相关类型定义。
 * 字段与后端 model.Agent 的 JSON 序列化结果保持一致。
 */

export type AgentStatus = 'online' | 'offline'

/** Agent 标签（键值对） */
export type AgentLabels = Record<string, string>

/** Agent 实体 */
export interface Agent {
  id: string
  hostname: string
  ip: string
  internal_ip: string
  os: string
  os_version: string
  kernel_version: string
  cpu_cores: number
  memory_total: number
  disk_total: number
  arch: string
  agent_version: string
  last_heartbeat_at: number
  status: string
  lite_mode: boolean
  created_at: string
  updated_at: string
}

/** GET /api/agents 返回的分页结构（data 字段） */
export interface AgentListResult {
  agents: Agent[]
  total: number
  page: number
  page_size: number
}

/** GET /api/agents/:id 返回的详情结构（data 字段） */
export interface AgentDetailResult {
  agent: Agent
  labels: AgentLabels
  /** Redis 中的实时指标哈希（字符串值），仅作兜底使用 */
  realtime: Record<string, string>
}
