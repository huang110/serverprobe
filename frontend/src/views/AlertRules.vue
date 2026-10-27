<script setup lang="ts">
/**
 * 告警规则管理页。
 * - 创建规则（弹窗表单）
 * - 编辑规则（复用表单，回填数据，PUT 提交）
 * - 状态列点击直接启用/停用（PUT）
 * - 删除规则（确认弹窗，DELETE）
 */
import { onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import BaseModal from '@/components/BaseModal.vue'
import LoadingSpinner from '@/components/LoadingSpinner.vue'
import EmptyState from '@/components/EmptyState.vue'
import SeverityBadge from '@/components/SeverityBadge.vue'
import { createRule, deleteRule, listRules, updateRule } from '@/api/alert'
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
const deletingId = ref<number | null>(null)
const deleteConfirmVisible = ref(false)
const pendingDeleteRule = ref<AlertRule | null>(null)
/** 正在切换启停状态的规则 id */
const togglingId = ref<number | null>(null)

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

/** 当前编辑的规则 id；null 表示创建模式 */
const editingId = ref<number | null>(null)
const modalTitle = () => (editingId.value === null ? '创建告警规则' : '编辑告警规则')

function resetForm() {
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
}

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
  editingId.value = null
  resetForm()
  modalVisible.value = true
}

function openEdit(rule: AlertRule) {
  editingId.value = rule.id
  Object.assign(form, {
    name: rule.name,
    agent_id: rule.agent_id || '',
    metric: rule.metric,
    condition: rule.condition,
    threshold: rule.threshold,
    duration: rule.duration,
    severity: rule.severity,
    webhook_url: rule.webhook_url || '',
    enabled: rule.enabled
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

function buildPayload() {
  return {
    name: form.name.trim(),
    agent_id: form.agent_id,
    metric: form.metric,
    condition: form.condition,
    threshold: Number(form.threshold),
    duration: Number(form.duration),
    severity: form.severity,
    webhook_url: form.webhook_url.trim(),
    enabled: form.enabled
  }
}

async function handleSubmit() {
  if (!form.name.trim()) {
    toastStore.warning('请填写规则名称')
    return
  }
  submitting.value = true
  try {
    if (editingId.value === null) {
      const created = await createRule(buildPayload())
      rules.value.unshift(created)
      toastStore.success('告警规则已创建')
    } else {
      const updated = await updateRule(editingId.value, buildPayload())
      const idx = rules.value.findIndex((r) => r.id === editingId.value)
      if (idx >= 0) rules.value[idx] = updated
      toastStore.success('告警规则已更新')
    }
    modalVisible.value = false
  } catch (err) {
    toastStore.danger(
      editingId.value === null ? '创建失败' : '更新失败',
      err instanceof Error ? err.message : ''
    )
  } finally {
    submitting.value = false
  }
}

/** 点击状态列：切换启用/停用 */
async function toggleEnabled(rule: AlertRule) {
  if (togglingId.value !== null) return
  togglingId.value = rule.id
  const nextEnabled = !rule.enabled
  try {
    const updated = await updateRule(rule.id, {
      name: rule.name,
      agent_id: rule.agent_id || '',
      metric: rule.metric,
      condition: rule.condition,
      threshold: rule.threshold,
      duration: rule.duration,
      severity: rule.severity,
      webhook_url: rule.webhook_url || '',
      enabled: nextEnabled
    })
    const idx = rules.value.findIndex((r) => r.id === rule.id)
    if (idx >= 0) rules.value[idx] = updated
    toastStore.success(nextEnabled ? '规则已启用' : '规则已停用')
  } catch (err) {
    toastStore.danger('状态切换失败', err instanceof Error ? err.message : '')
  } finally {
    togglingId.value = null
  }
}

function askDelete(rule: AlertRule) {
  pendingDeleteRule.value = rule
  deleteConfirmVisible.value = true
}

async function confirmDelete() {
  const rule = pendingDeleteRule.value
  if (!rule) return
  deletingId.value = rule.id
  try {
    await deleteRule(rule.id)
    rules.value = rules.value.filter((r) => r.id !== rule.id)
    deleteConfirmVisible.value = false
    toastStore.success('告警规则已删除')
  } catch (err) {
    toastStore.danger('删除失败', err instanceof Error ? err.message : '')
  } finally {
    deletingId.value = null
    pendingDeleteRule.value = null
  }
}

onMounted(async () => {
  window.addEventListener('app:refresh', loadRules)
  await Promise.all([
    loadRules(),
    listAllAgents()
      .then((list) => (agents.value = list))
      .catch(() => {})
  ])
})

onBeforeUnmount(() => window.removeEventListener('app:refresh', loadRules))
</script>

<template>
  <div>
    <!-- 工具栏 -->
    <div class="mb-6 flex flex-wrap items-center gap-3">
      <div>
        <h1 class="text-xl font-bold text-content">告警规则</h1>
        <p class="mt-0.5 text-xs text-muted">配置指标阈值触发条件与 Webhook 通知</p>
      </div>
      <div class="ml-auto flex gap-2">
        <button
          class="btn-ghost px-3 py-1.5 text-xs"
          aria-label="刷新告警规则列表（快捷键 R）"
          @click="loadRules"
        >
          <svg class="h-4 w-4" :class="loading && 'animate-spin'" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
          </svg>
          刷新
        </button>
        <button class="btn-primary px-3 py-1.5 text-xs" aria-label="创建新的告警规则" @click="openCreate">
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
        <table class="data-table stack">
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
              <span class="row-indicator"></span>
              <td data-label="规则名称" class="font-medium text-content">{{ rule.name }}</td>
              <td data-label="目标 Agent">
                <span :class="rule.agent_id ? '' : 'text-accent'">
                  {{ agentName(rule.agent_id) }}
                </span>
              </td>
              <td data-label="指标" class="whitespace-nowrap">{{ metricLabel(rule.metric) }}</td>
              <td data-label="条件" class="font-mono text-xs">{{ conditionLabel(rule.condition) }}</td>
              <td data-label="阈值" class="font-mono text-xs">{{ rule.threshold }}</td>
              <td data-label="持续时间" class="whitespace-nowrap">{{ formatSeconds(rule.duration) }}</td>
              <td data-label="严重级别"><SeverityBadge :severity="rule.severity" /></td>
              <td data-label="状态">
                <button
                  type="button"
                  class="inline-flex items-center gap-1 rounded px-1 py-0.5 text-xs transition hover:bg-surface-3"
                  :aria-label="rule.enabled ? '点击停用该规则' : '点击启用该规则'"
                  :disabled="togglingId === rule.id"
                  @click="toggleEnabled(rule)"
                >
                  <span
                    class="h-1.5 w-1.5 rounded-full"
                    :class="[
                      togglingId === rule.id
                        ? 'animate-pulse bg-warning'
                        : rule.enabled
                          ? 'bg-success'
                          : 'bg-muted'
                    ]"
                  ></span>
                  {{ togglingId === rule.id ? '...' : rule.enabled ? '启用' : '停用' }}
                </button>
              </td>
              <td data-label="创建时间" class="whitespace-nowrap text-xs text-muted">{{ formatDateTime(rule.created_at) }}</td>
              <td data-label="操作">
                <div class="flex justify-end gap-1.5">
                  <button aria-label="编辑规则" class="rounded-md px-2 py-1 text-xs text-muted transition hover:bg-surface-3 hover:text-accent" @click="openEdit(rule)">编辑</button>
                  <button aria-label="删除规则" class="rounded-md px-2 py-1 text-xs text-muted transition hover:bg-surface-3 hover:text-danger" @click="askDelete(rule)">删除</button>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- 创建 / 编辑规则弹窗 -->
    <BaseModal :show="modalVisible" :title="modalTitle()" @close="modalVisible = false">
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
            role="switch"
            :aria-checked="form.enabled"
            aria-label="启用或停用该规则"
            class="relative h-6 w-11 rounded-full transition"
            :class="form.enabled ? 'bg-success' : 'bg-surface-3'"
            @click="form.enabled = !form.enabled"
          >
            <span
              class="absolute top-0.5 h-5 w-5 rounded-full bg-white shadow transition-all"
              :class="form.enabled ? 'left-[22px]' : 'left-0.5'"
            ></span>
          </button>
          <span class="text-sm text-content/90">启用该规则</span>
        </div>
      </div>

      <template #footer>
        <button class="btn-ghost" @click="modalVisible = false">取消</button>
        <button class="btn-primary" :disabled="submitting" @click="handleSubmit">
          {{ submitting ? (editingId === null ? '创建中...' : '保存中...') : editingId === null ? '创建' : '保存' }}
        </button>
      </template>
    </BaseModal>

    <!-- 删除确认弹窗 -->
    <BaseModal :show="deleteConfirmVisible" title="删除确认" width="420px" @close="deleteConfirmVisible = false">
      <p class="text-sm leading-relaxed text-muted">
        确定要删除告警规则
        <span class="font-semibold text-content">{{ pendingDeleteRule?.name }}</span>
        吗？删除后该规则将不再触发，且不可恢复。
      </p>
      <template #footer>
        <button class="btn-ghost" @click="deleteConfirmVisible = false">取消</button>
        <button class="btn-danger" :disabled="deletingId !== null" @click="confirmDelete">
          {{ deletingId !== null ? '删除中...' : '确认删除' }}
        </button>
      </template>
    </BaseModal>
  </div>
</template>
