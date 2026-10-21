<script setup lang="ts">
/**
 * 告警规则管理页。
 * 说明：当前后端仅提供规则的「查询」与「创建」接口（无更新/删除），
 *      因此表格中的编辑/删除按钮会提示接口暂未开放。
 */
import { onMounted, reactive, ref } from 'vue'
import BaseModal from '@/components/BaseModal.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import EmptyState from '@/components/EmptyState.vue'
import SeverityBadge from '@/components/SeverityBadge.vue'
import { createRule, listRules } from '@/api/alert'
import { listAllAgents } from '@/api/agent'
import { useToastStore } from '@/stores/toast'
import { formatDateTime, formatSeconds } from '@/utils/format'
import type { AlertRule } from '@/types/alert'
import type { Agent } from '@/types/agent'

const toastStore = useToastStore()

const rules = ref<AlertRule[]>([])
const agents = ref<Agent[]>([])
const loading = ref(false)
const modalVisible = ref(false)
const submitting = ref(false)

// 指标与条件选项
const metricOptions = [
  { value: 'cpu_usage', label: 'CPU 使用率' },
  { value: 'memory_percent', label: '内存使用率' },
  { value: 'disk_usage', label: '磁盘使用率' },
  { value: 'load1', label: '系统负载 (load1)' }
]
const conditionOptions = [
  { value: 'gt', label: '> 大于' },
  { value: 'gte', label: '>= 大于等于' },
  { value: 'lt', label: '< 小于' },
  { value: 'lte', label: '<= 小于等于' },
  { value: 'eq', label: '= 等于' }
]
const severityOptions = [
  { value: 'warning', label: '警告 warning' },
  { value: 'critical', label: '严重 critical' }
]

const form = reactive({
  name: '',
  agent_id: '',
  metric: 'cpu_usage',
  condition: 'gt',
  threshold: 80,
  duration: 300,
  severity: 'warning',
  webhook_url: '',
  enabled: true
})

async function loadRules() {
  loading.value = true
  try {
    rules.value = await listRules()
  } catch (err) {
    toastStore.danger('加载告警规则失败', err instanceof Error ? err.message : '')
  } finally {
    loading.value = false
  }
}

function openCreate() {
  Object.assign(form, {
    name: '',
    agent_id: '',
    metric: 'cpu_usage',
    condition: 'gt',
    threshold: 80,
    duration: 300,
    severity: 'warning',
    webhook_url: '',
    enabled: true
  })
  modalVisible.value = true
}

function metricLabel(value: string): string {
  return metricOptions.find((m) => m.value === value)?.label || value
}

function conditionLabel(value: string): string {
  return conditionOptions.find((c) => c.value === value)?.label || value
}

function agentName(agentId: string): string {
  if (!agentId) return '全局'
  return agents.value.find((a) => a.id === agentId)?.hostname || agentId.slice(0, 8)
}

async function handleSubmit() {
  if (!form.name.trim()) {
    toastStore.warning('请填写规则名称')
    return
  }
  submitting.value = true
  try {
    const created = await createRule({
      name: form.name.trim(),
      agent_id: form.agent_id,
      metric: form.metric,
      condition: form.condition,
      threshold: Number(form.threshold),
      duration: Number(form.duration),
      severity: form.severity,
      webhook_url: form.webhook_url.trim(),
      enabled: form.enabled
    })
    rules.value.unshift(created)
    modalVisible.value = false
    toastStore.success('告警规则已创建')
  } catch (err) {
    toastStore.danger('创建失败', err instanceof Error ? err.message : '')
  } finally {
    submitting.value = false
  }
}

function notSupported(action: string) {
  toastStore.info(`暂不支持${action}`, '当前后端未开放规则的更新/删除接口')
}

onMounted(async () => {
  await Promise.all([loadRules(), listAllAgents().then((list) => (agents.value = list)).catch(() => {})])
})
</script>

<template>
  <div>
    <!-- 工具栏 -->
    <div class="mb-6 flex flex-wrap items-center gap-3">
      <div>
        <h1 class="text-xl font-bold text-white">告警规则</h1>
        <p class="mt-0.5 text-xs text-muted">配置指标阈值触发条件与 Webhook 通知</p>
      </div>
      <div class="ml-auto flex gap-2">
        <button class="btn-ghost px-3 py-1.5 text-xs" @click="loadRules">刷新</button>
        <button class="btn-primary px-3 py-1.5 text-xs" @click="openCreate">
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M12 4.5v15m7.5-7.5h-15" />
          </svg>
          创建规则
        </button>
      </div>
    </div>

    <div class="card overflow-hidden">
      <LoadingSpinner v-if="loading" />
      <EmptyState v-else-if="rules.length === 0" text="暂无告警规则" hint="点击右上角「创建规则」添加第一条规则" />
      <div v-else class="overflow-x-auto">
        <table class="data-table">
          <thead>
            <tr>
              <th>规则名称</th>
              <th>目标 Agent</th>
              <th>指标</th>
              <th>条件</th>
              <th>阈值</th>
              <th>持续时间</th>
              <th>严重级别</th>
              <th>状态</th>
              <th>创建时间</th>
              <th class="text-right">操作</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in rules" :key="rule.id">
              <td class="font-medium text-slate-100">{{ rule.name }}</td>
              <td>
                <span :class="rule.agent_id ? '' : 'text-accent'">
                  {{ agentName(rule.agent_id) }}
                </span>
              </td>
              <td class="whitespace-nowrap">{{ metricLabel(rule.metric) }}</td>
              <td class="font-mono text-xs">{{ conditionLabel(rule.condition) }}</td>
              <td class="font-mono text-xs">{{ rule.threshold }}</td>
              <td class="whitespace-nowrap">{{ formatSeconds(rule.duration) }}</td>
              <td><SeverityBadge :severity="rule.severity" /></td>
              <td>
                <span
                  class="inline-flex items-center gap-1 text-xs"
                  :class="rule.enabled ? 'text-success' : 'text-muted'"
                >
                  <span class="h-1.5 w-1.5 rounded-full" :class="rule.enabled ? 'bg-success' : 'bg-slate-500'"></span>
                  {{ rule.enabled ? '启用' : '停用' }}
                </span>
              </td>
              <td class="whitespace-nowrap text-xs text-muted">{{ formatDateTime(rule.created_at) }}</td>
              <td>
                <div class="flex justify-end gap-1.5">
                  <button class="rounded-md px-2 py-1 text-xs text-muted transition hover:bg-ink-600 hover:text-accent" @click="notSupported('编辑规则')">编辑</button>
                  <button class="rounded-md px-2 py-1 text-xs text-muted transition hover:bg-ink-600 hover:text-danger" @click="notSupported('删除规则')">删除</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 创建规则弹窗 -->
    <BaseModal :show="modalVisible" title="创建告警规则" @close="modalVisible = false">
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
        <div class="sm:col-span-2">
          <label class="form-label">规则名称 *</label>
          <input v-model="form.name" type="text" class="form-input" placeholder="如：CPU 高负载告警" />
        </div>

        <div>
          <label class="form-label">目标 Agent</label>
          <select v-model="form.agent_id" class="form-input">
            <option value="">全局（所有 Agent）</option>
            <option v-for="a in agents" :key="a.id" :value="a.id">{{ a.hostname }}</option>
          </select>
        </div>

        <div>
          <label class="form-label">监控指标</label>
          <select v-model="form.metric" class="form-input">
            <option v-for="m in metricOptions" :key="m.value" :value="m.value">{{ m.label }}</option>
          </select>
        </div>

        <div>
          <label class="form-label">触发条件</label>
          <select v-model="form.condition" class="form-input">
            <option v-for="c in conditionOptions" :key="c.value" :value="c.value">{{ c.label }}</option>
          </select>
        </div>

        <div>
          <label class="form-label">阈值</label>
          <input v-model.number="form.threshold" type="number" step="0.1" class="form-input" />
        </div>

        <div>
          <label class="form-label">持续时间（秒）</label>
          <input v-model.number="form.duration" type="number" min="1" class="form-input" />
        </div>

        <div>
          <label class="form-label">严重级别</label>
          <select v-model="form.severity" class="form-input">
            <option v-for="s in severityOptions" :key="s.value" :value="s.value">{{ s.label }}</option>
          </select>
        </div>

        <div class="sm:col-span-2">
          <label class="form-label">Webhook URL（可选）</label>
          <input v-model="form.webhook_url" type="text" class="form-input" placeholder="https://example.com/webhook" />
        </div>

        <div class="flex items-center gap-2 sm:col-span-2">
          <button
            type="button"
            class="relative h-6 w-11 rounded-full transition"
            :class="form.enabled ? 'bg-success' : 'bg-ink-500'"
            @click="form.enabled = !form.enabled"
          >
            <span
              class="absolute top-0.5 h-5 w-5 rounded-full bg-white transition-all"
              :class="form.enabled ? 'left-[22px]' : 'left-0.5'"
            ></span>
          </button>
          <span class="text-sm text-slate-300">启用该规则</span>
        </div>
      </div>

      <template #footer>
        <button class="btn-ghost" @click="modalVisible = false">取消</button>
        <button class="btn-primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? '创建中...' : '创建' }}
        </button>
      </template>
    </BaseModal>
  </div>
</template>
