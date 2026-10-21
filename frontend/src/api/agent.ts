/** Agent 管理接口 */
import apiClient from './client'
import type {
  Agent,
  AgentDetailResult,
  AgentLabels,
  AgentListResult
} from '@/types/agent'

export interface ListAgentsParams {
  page?: number
  page_size?: number
  status?: 'all' | 'online' | 'offline'
  keyword?: string
}

/** GET /api/agents 分页查询 Agent 列表 */
export async function listAgents(params: ListAgentsParams = {}): Promise<AgentListResult> {
  const { data } = await apiClient.get('/api/agents', { params })
  if (!data?.success) throw new Error(data?.message || '获取 Agent 列表失败')
  return data.data as AgentListResult
}

/**
 * 拉取全部 Agent（自动翻页）。
 * 后端单页最多返回 100 条，此处循环加载直到取完。
 */
export async function listAllAgents(keyword = ''): Promise<Agent[]> {
  const pageSize = 100
  const collected: Agent[] = []
  let page = 1

  while (true) {
    const result = await listAgents({ page, page_size: pageSize, keyword })
    collected.push(...result.agents)
    if (collected.length >= result.total || result.agents.length === 0) break
    page += 1
  }
  return collected
}

/** GET /api/agents/:id 查询 Agent 详情 */
export async function getAgent(id: string): Promise<AgentDetailResult> {
  const { data } = await apiClient.get(`/api/agents/${id}`)
  if (!data?.success) throw new Error(data?.message || '获取 Agent 详情失败')
  return data.data as AgentDetailResult
}

/** PUT /api/agents/:id 更新 Agent 标签 */
export async function updateAgentLabels(id: string, labels: AgentLabels): Promise<void> {
  const { data } = await apiClient.put(`/api/agents/${id}`, { labels })
  if (!data?.success) throw new Error(data?.message || '更新失败')
}

/** DELETE /api/agents/:id 删除 Agent */
export async function deleteAgent(id: string): Promise<void> {
  const { data } = await apiClient.delete(`/api/agents/${id}`)
  if (!data?.success) throw new Error(data?.message || '删除失败')
}
