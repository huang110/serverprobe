/** 告警规则与记录接口 */
import apiClient from './client'
import type {
  AlertRecord,
  AlertRule,
  CreateRulePayload
} from '@/types/alert'

export interface ListRulesParams {
  agent_id?: string
  enabled?: 'all' | 'true' | 'false'
}

/** GET /api/alert/rules 查询告警规则 */
export async function listRules(params: ListRulesParams = {}): Promise<AlertRule[]> {
  const { data } = await apiClient.get('/api/alert/rules', { params })
  if (!data?.success) throw new Error(data?.message || '获取告警规则失败')
  return (data.data as AlertRule[]) || []
}

/** POST /api/alert/rules 创建告警规则 */
export async function createRule(payload: CreateRulePayload): Promise<AlertRule> {
  const { data } = await apiClient.post('/api/alert/rules', payload)
  if (!data?.success) throw new Error(data?.message || '创建告警规则失败')
  return data.data as AlertRule
}

/** PUT /api/alert/rules/:id 更新告警规则（含 enabled 启停切换） */
export async function updateRule(
  id: number,
  payload: CreateRulePayload
): Promise<AlertRule> {
  const { data } = await apiClient.put(`/api/alert/rules/${id}`, payload)
  if (!data?.success) throw new Error(data?.message || '更新告警规则失败')
  return data.data as AlertRule
}

/** DELETE /api/alert/rules/:id 删除告警规则 */
export async function deleteRule(id: number): Promise<void> {
  const { data } = await apiClient.delete(`/api/alert/rules/${id}`)
  if (!data?.success) throw new Error(data?.message || '删除告警规则失败')
}

export interface ListRecordsParams {
  agent_id?: string
  severity?: string
  status?: string
  start?: number
  end?: number
}

/**
 * GET /api/alert/records 查询告警记录。
 * 后端返回最近时间范围内（默认 24 小时）最多 100 条的数组。
 */
export async function listRecords(params: ListRecordsParams = {}): Promise<AlertRecord[]> {
  const { data } = await apiClient.get('/api/alert/records', { params })
  if (!data?.success) throw new Error(data?.message || '获取告警记录失败')
  return (data.data as AlertRecord[]) || []
}
