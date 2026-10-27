<script setup lang="ts">
/**
 * 发行版 / 操作系统徽章。
 * 根据 agent.os / agent.os_version 识别常见发行版，展示彩色胶囊标签 + 简约 SVG 图标：
 * Alpine / Debian / Ubuntu / CentOS / Rocky / Alma / Fedora / Arch / Windows / macOS。
 */
import { computed } from 'vue'
import type { Agent } from '@/types/agent'

const props = defineProps<{ agent: Agent }>()

interface Distro {
  key: string
  label: string
  /** Tailwind 文本/背景色调（胶囊） */
  tone: string
  /** 简化 SVG path（24x24） */
  path: string
}

const distro = computed<Distro>(() => {
  const hay = `${props.agent.os || ''} ${props.agent.os_version || ''}`.toLowerCase()

  if (hay.includes('alpine'))
    return { key: 'alpine', label: 'Alpine', tone: 'text-sky-300 bg-sky-500/15 border-sky-500/30', path: 'M12 3l7 4v10l-7 4-7-4V7l7-4zm0 2.3L7 8.2v7.6l5 2.9 5-2.9V8.2l-5-2.9zM12 8v8' }
  if (hay.includes('ubuntu'))
    return { key: 'ubuntu', label: 'Ubuntu', tone: 'text-orange-300 bg-orange-500/15 border-orange-500/30', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zm0 2.2l5.4 3.1v6.3L12 17.7 6.6 14.6V8.3L12 5.2zM9.5 9.5c1.4 0 2.5 1.1 2.5 2.5s-1.1 2.5-2.5 2.5S7 13.4 7 12s1.1-2.5 2.5-2.5z' }
  if (hay.includes('debian'))
    return { key: 'debian', label: 'Debian', tone: 'text-rose-300 bg-rose-500/15 border-rose-500/30', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zm-1 4l4 2v6l-4 2V7zm-3 7c.8 0 1.5.7 1.5 1.5S8.8 17 8 17s-1.5-.7-1.5-1.5S7.2 14 8 14z' }
  if (hay.includes('rocky'))
    return { key: 'rocky', label: 'Rocky', tone: 'text-emerald-300 bg-emerald-500/15 border-emerald-500/30', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zm0 2.5a6.5 6.5 0 016.3 5h-3.4a3.3 3.3 0 00-4.7-2.2l-1.5-2.6c1-.7 2.1-1 3.3-1zM6.5 9h6l-1.7 3H6.7a6.5 6.5 0 01-.2-3zm3.8 4l1.7 3a3.3 3.3 0 003.3-1.7h2.2a6.5 6.5 0 01-7.2 4.5v-5.8z' }
  if (hay.includes('almalinux') || hay.includes('alma'))
    return { key: 'alma', label: 'Alma', tone: 'text-indigo-300 bg-indigo-500/15 border-indigo-500/30', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zm-3 4h6v2h-2v6h-2V9H9V7z' }
  if (hay.includes('centos') || hay.includes('cent os'))
    return { key: 'centos', label: 'CentOS', tone: 'text-violet-300 bg-violet-500/15 border-violet-500/30', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zm-1 3.3v3.2H7.5a4.5 4.5 0 00-2 3.3H3.3a8.7 8.7 0 017.7-6.5zm2 0a8.7 8.7 0 017.7 6.5h-2.2a4.5 4.5 0 00-2-3.3H13V6.3zM7 14h3.3v3.2a4.5 4.5 0 01-3.3-3.2zm6.7 0H17a4.5 4.5 0 01-3.3 3.2V14z' }
  if (hay.includes('fedora'))
    return { key: 'fedora', label: 'Fedora', tone: 'text-blue-300 bg-blue-500/15 border-blue-500/30', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zm3 4.5c-2.2 0-3.8 1.2-4.3 3.2l-1.7 4.3h2.2l1.5-3.7c.3-.8 1-1.3 1.9-1.3h1V7.5h-.6z' }
  if (hay.includes('arch'))
    return { key: 'arch', label: 'Arch', tone: 'text-cyan-300 bg-cyan-500/15 border-cyan-500/30', path: 'M3 18c3-1 5.5-3 7-6l5 2c1 2.3 3 4 6 5v2H3v-3zm9-8l3 1.2L10 5 8 8.5l4 1.5z' }
  if (hay.includes('windows'))
    return { key: 'windows', label: 'Windows', tone: 'text-sky-300 bg-sky-500/15 border-sky-500/30', path: 'M3 5.5L10.5 4v7H3V5.5zm0 13L10.5 19v-7H3v6.5zM11.5 3.9L21 3v8h-9.5V3.9zm0 16.2V13H21v8l-9.5-.9z' }
  if (hay.includes('mac') || hay.includes('darwin'))
    return { key: 'macos', label: 'macOS', tone: 'text-slate-200 bg-slate-400/15 border-slate-400/30', path: 'M16.4 13.4c0-2.3 1.9-3.4 2-3.4-1.1-1.6-2.8-1.8-3.4-1.8-1.4-.1-2.8.8-3.5.8-.7 0-1.9-.8-3.1-.8-1.6 0-3 .9-3.9 2.4-1.6 2.9-.4 7.1 1.2 9.4.8 1.1 1.7 2.4 2.9 2.3 1.2 0 1.6-.7 3-.7s1.8.7 3 .7c1.2 0 2-1.1 2.8-2.3.9-1.3 1.2-2.6 1.2-2.7-.1 0-2.3-.9-2.2-3.2zM14.2 6.6c.6-.8 1-1.9.9-3-.9 0-2 .6-2.7 1.4-.6.7-1.1 1.8-.9 2.8 1 .1 2-.5 2.7-1.2z' }

  // 通用 Linux
  if (hay.includes('linux') || !props.agent.os)
    return { key: 'linux', label: 'Linux', tone: 'text-yellow-200 bg-yellow-500/15 border-yellow-500/30', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zm-3 5c.8 0 1.5.7 1.5 1.5S9.8 11 9 11s-1.5-.7-1.5-1.5S8.2 8 9 8zm6 0c.8 0 1.5.7 1.5 1.5S15.8 11 15 11s-1.5-.7-1.5-1.5S14.2 8 15 8zM8 13h8c0 2-1.8 3.5-4 3.5S8 15 8 13z' }

  return { key: 'other', label: props.agent.os || 'OS', tone: 'text-muted bg-surface-2 border-line', path: 'M12 3a9 9 0 100 18 9 9 0 000-18zM7 10h10v1.5H7V10zm0 3h7v1.5H7V13z' }
})
</script>

<template>
  <span
    class="inline-flex items-center gap-1 rounded-full border px-1.5 py-0.5 text-[10px] font-semibold leading-none"
    :class="distro.tone"
    :title="`${agent.os} ${agent.os_version}`"
  >
    <svg class="h-3 w-3" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
      <path :d="distro.path" />
    </svg>
    {{ distro.label }}
  </span>
</template>
