/**
 * 告警相关类型定义，对应后端 model.AlertRule / model.AlertRecord。
 */

export type AlertSeverity = 'info' | 'warning' | 'critical'
export type AlertStatus = 'firing' | 'resolved'

/** 告警规则监控指标（后端白名单） */
export type RuleMetric = 'cpu_usage' | 'memory_percent' | 'disk_usage' | 'load1'

/** 告警触发条件（后端白名单） */
export type RuleCondition = 'gt' | 'gte' | 'lt' | 'lte' | 'eq'

/** 告警规则 */
export interface AlertRule {
  id: number
  name: string
  /** 空字符串表示全局规则 */
  agent_id: string
  metric: string
  condition: string
  threshold: number
  duration: number
  severity: string
  enabled: boolean
  expression: string
  notify_url: string
  webhook_url: string
  message: string
  created_at: string
  updated_at: string
}

/** 创建告警规则的请求体 */
export interface CreateRulePayload {
  name: string
  agent_id: string
  metric: string
  condition: string
  threshold: number
  duration: number
  severity: string
  webhook_url: string
  enabled: boolean
}

/** 告警记录 */
export interface AlertRecord {
  id: number
  rule_id: number
  agent_id: string
  metric: string
  threshold: number
  actual_value: number
  severity: string
  message: string
  status: string
  fired_at: number
  resolved_at: number
  notified_at: number
  created_at: string
}
