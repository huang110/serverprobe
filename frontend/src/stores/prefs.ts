/**
 * 用户偏好状态管理。
 * 包含：主题（dark/light/auto）、对比度、密度、刷新频率、卡片指标、视图模式。
 * 持久化到 localStorage，并立即同步到 <html> 的 data-* 属性（无闪烁）。
 */
import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'

export type ThemeMode = 'dark' | 'light' | 'auto'
export type ContrastMode = 'normal' | 'high'
export type DensityMode = 'compact' | 'comfortable' | 'spacious'
export type ViewMode = 'grid' | 'compact'

const PREFS_KEY = 'prefs'

interface PersistedPrefs {
  theme: ThemeMode
  contrast: ContrastMode
  density: DensityMode
  refreshInterval: number
  cardMetrics: string[]
  viewMode: ViewMode
  /** 是否在界面上打码隐藏 IP（晒机截图用） */
  hideIp: boolean
}

const DEFAULTS: PersistedPrefs = {
  theme: 'dark',
  contrast: 'normal',
  density: 'comfortable',
  refreshInterval: 5,
  cardMetrics: ['cpu', 'memory', 'load', 'network'],
  viewMode: 'grid',
  hideIp: false
}

function loadPrefs(): PersistedPrefs {
  try {
    const raw = localStorage.getItem(PREFS_KEY)
    if (!raw) return { ...DEFAULTS }
    return { ...DEFAULTS, ...(JSON.parse(raw) as Partial<PersistedPrefs>) }
  } catch {
    return { ...DEFAULTS }
  }
}

function resolveTheme(theme: ThemeMode): 'dark' | 'light' {
  if (theme === 'auto') {
    return window.matchMedia('(prefers-color-scheme: light)').matches
      ? 'light'
      : 'dark'
  }
  return theme
}

export const usePrefsStore = defineStore('prefs', () => {
  const initial = loadPrefs()

  const theme = ref<ThemeMode>(initial.theme)
  const contrast = ref<ContrastMode>(initial.contrast)
  const density = ref<DensityMode>(initial.density)
  const refreshInterval = ref<number>(initial.refreshInterval)
  const cardMetrics = ref<string[]>(initial.cardMetrics)
  const viewMode = ref<ViewMode>(initial.viewMode)
  const hideIp = ref<boolean>(initial.hideIp)

  /** 当前实际生效的主题（auto 解析后） */
  const resolvedTheme = computed<'dark' | 'light'>(() => resolveTheme(theme.value))
  const isDark = computed(() => resolvedTheme.value === 'dark')

  /** 将偏好写入 <html> 属性，由 CSS 变量完成切换 */
  function applyToDocument() {
    const root = document.documentElement
    root.setAttribute('data-theme', resolvedTheme.value)
    root.setAttribute('data-contrast', contrast.value)
    root.setAttribute('data-density', density.value)
  }

  function persist() {
    const payload: PersistedPrefs = {
      theme: theme.value,
      contrast: contrast.value,
      density: density.value,
      refreshInterval: refreshInterval.value,
      cardMetrics: cardMetrics.value,
      viewMode: viewMode.value,
      hideIp: hideIp.value
    }
    localStorage.setItem(PREFS_KEY, JSON.stringify(payload))
  }

  // 任意偏好变化：同步 DOM + 持久化
  watch(
    [theme, contrast, density, refreshInterval, cardMetrics, viewMode, hideIp],
    () => {
      applyToDocument()
      persist()
    },
    { deep: true }
  )

  // 跟随系统主题（auto 模式下系统切换时立即响应）
  window
    .matchMedia('(prefers-color-scheme: light)')
    .addEventListener('change', () => {
      if (theme.value === 'auto') applyToDocument()
    })

  /** 切换某项卡片指标的显示开关 */
  function toggleCardMetric(metric: string) {
    const list = cardMetrics.value
    cardMetrics.value = list.includes(metric)
      ? list.filter((m) => m !== metric)
      : [...list, metric]
  }

  return {
    theme,
    contrast,
    density,
    refreshInterval,
    cardMetrics,
    viewMode,
    hideIp,
    resolvedTheme,
    isDark,
    applyToDocument,
    toggleCardMetric
  }
})
