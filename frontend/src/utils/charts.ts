/**
 * ECharts 通用配置工厂。
 * 颜色从 CSS 变量读取，随主题（深色/浅色/高对比度）变化。
 */
import type { EChartsOption } from 'echarts'
import type { MetricPoint } from '@/types/metrics'

export interface SeriesDef {
  name: string
  color: string
  points: MetricPoint[]
}

/** 读取一个「空格分隔 RGB」的 CSS 变量，返回 rgb() 字符串 */
function cssVar(name: string, alpha = 1): string {
  const raw = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim()
  if (!raw) return `rgb(${name === '--muted' ? '139 148 163' : '43 49 60'})`
  return `rgb(${raw}${alpha < 1 ? ` / ${alpha})` : ')'}`
}

/** 当数据点超过 1000 时自动降采样，保证图表渲染性能 */
function sample(points: MetricPoint[], max = 1000): Array<[number, number]> {
  if (points.length <= max) {
    return points.map((p) => [p.timestamp * 1000, Number(p.value?.toFixed(2))])
  }
  // LTTB 之外的轻量方案：等步长抽样，保留首尾
  const step = (points.length - 1) / (max - 1)
  const sampled: Array<[number, number]> = []
  for (let i = 0; i < max; i++) {
    const p = points[Math.round(i * step)]
    if (p) sampled.push([p.timestamp * 1000, Number(p.value?.toFixed(2))])
  }
  return sampled
}

function axisStyle() {
  return {
    axisLine: { lineStyle: { color: cssVar('--border') } },
    axisLabel: { color: cssVar('--muted'), fontSize: 11 },
    splitLine: { lineStyle: { color: cssVar('--border', 0.55), type: 'dashed' as const } }
  }
}

function tooltipConfig(unit: string) {
  return {
    trigger: 'axis' as const,
    backgroundColor: cssVar('--surface', 0.96),
    borderColor: cssVar('--border'),
    textStyle: { color: cssVar('--text'), fontSize: 12 },
    axisPointer: { type: 'line' as const, lineStyle: { color: cssVar('--accent') } },
    valueFormatter: (value: unknown) => `${value}${unit}`
  }
}

/**
 * 生成折线图配置。
 * @param title 图表标题
 * @param seriesList 一个或多个数据系列
 * @param unit Y 轴单位
 * @param percent 是否为 0-100 的百分比图
 */
export function buildLineOption(
  title: string,
  seriesList: SeriesDef[],
  unit = '',
  percent = false
): EChartsOption {
  return {
    backgroundColor: 'transparent',
    color: seriesList.map((s) => s.color),
    title: {
      text: title,
      left: 4,
      top: 2,
      textStyle: { color: cssVar('--text'), fontSize: 14, fontWeight: 600 }
    },
    tooltip: tooltipConfig(unit),
    legend: {
      data: seriesList.map((s) => s.name),
      top: 4,
      right: 8,
      textStyle: { color: cssVar('--muted') },
      itemWidth: 14,
      itemHeight: 8
    },
    grid: { left: 48, right: 20, top: 48, bottom: 42 },
    xAxis: {
      type: 'time',
      ...axisStyle()
    },
    yAxis: {
      type: 'value',
      min: percent ? 0 : undefined,
      max: percent ? 100 : undefined,
      ...axisStyle(),
      axisLabel: {
        ...axisStyle().axisLabel,
        formatter: percent ? '{value}%' : `{value}${unit}`
      }
    },
    series: seriesList.map((s) => ({
      name: s.name,
      type: 'line' as const,
      smooth: true,
      showSymbol: false,
      symbol: 'circle',
      symbolSize: 5,
      lineStyle: { width: 2, color: s.color },
      itemStyle: { color: s.color },
      areaStyle:
        seriesList.length === 1
          ? {
              color: {
                type: 'linear' as const,
                x: 0,
                y: 0,
                x2: 0,
                y2: 1,
                colorStops: [
                  { offset: 0, color: s.color + '40' },
                  { offset: 1, color: s.color + '00' }
                ]
              }
            }
          : undefined,
      data: sample(s.points)
    }))
  }
}

/** TCP Ping 多目标历史序列（点携带抖动与丢包率） */
export interface LatencySeriesDef {
  name: string
  color: string
  points: import('@/types/metrics').LatencyPoint[]
}

interface LatencySample {
  timestamp: number
  latency_ms: number
  jitter_ms: number
  packet_loss: number
}

/** TCP Ping 数据降采样（保留每点的抖动 / 丢包率），最多 1000 点 */
function sampleLatency(points: LatencySample[], max = 1000): LatencySample[] {
  if (points.length <= max) return points
  const step = (points.length - 1) / (max - 1)
  const out: LatencySample[] = []
  for (let i = 0; i < max; i++) {
    const p = points[Math.round(i * step)]
    if (p) {
      out.push({
        timestamp: p.timestamp,
        latency_ms: Number(p.latency_ms?.toFixed(2)),
        jitter_ms: Number(p.jitter_ms?.toFixed(2)),
        packet_loss: Number(p.packet_loss?.toFixed(1))
      })
    }
  }
  return out
}

/**
 * 生成多目标 TCP Ping 延迟折线图配置。
 * 多条不同颜色平滑曲线共用时间轴，单位 ms；Legend 可点击单选 / 多选线路。
 * 100% 丢包（latency_ms 为 0）的点断开为缺口，避免曲线被拉到 0。
 */
export function buildLatencyOption(
  title: string,
  seriesList: LatencySeriesDef[]
): EChartsOption {
  return {
    backgroundColor: 'transparent',
    color: seriesList.map((s) => s.color),
    title: {
      text: title,
      left: 4,
      top: 2,
      textStyle: { color: cssVar('--text'), fontSize: 14, fontWeight: 600 }
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: cssVar('--surface', 0.96),
      borderColor: cssVar('--border'),
      textStyle: { color: cssVar('--text'), fontSize: 12 },
      axisPointer: { type: 'line', lineStyle: { color: cssVar('--accent') } },
      formatter: (params: unknown) => {
        const list = params as Array<{
          seriesName: string
          color: string
          value: [number, number, number, number]
        }>
        if (!Array.isArray(list) || !list.length) return ''
        const time = new Date(list[0].value[0]).toLocaleString()
        const rows = list
          .map((p) => {
            const [, latency, jitter, loss] = p.value
            if (loss >= 100) {
              return `<div style="display:flex;align-items:center;gap:6px;margin-top:3px;">
                <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${p.color}"></span>
                <span>${p.seriesName}：<b style="color:${cssVar('--danger')}">超时 / 丢包</b></span>
              </div>`
            }
            return `<div style="display:flex;align-items:center;gap:6px;margin-top:3px;">
              <span style="display:inline-block;width:8px;height:8px;border-radius:50%;background:${p.color}"></span>
              <span>${p.seriesName}：<b>${latency}ms</b>
                <span style="color:${cssVar('--muted')}">抖动 ${jitter}ms · 丢包 ${loss}%</span>
              </span>
            </div>`
          })
          .join('')
        return `<div style="font-size:11px;color:${cssVar('--muted')}">${time}</div>${rows}`
      }
    },
    legend: {
      type: 'scroll',
      data: seriesList.map((s) => s.name),
      top: 4,
      right: 8,
      textStyle: { color: cssVar('--muted') },
      itemWidth: 14,
      itemHeight: 8
    },
    grid: { left: 48, right: 20, top: 48, bottom: 42 },
    xAxis: {
      type: 'time',
      ...axisStyle()
    },
    yAxis: {
      type: 'value',
      min: 0,
      ...axisStyle(),
      axisLabel: {
        ...axisStyle().axisLabel,
        formatter: '{value}ms'
      }
    },
    series: seriesList.map((s) => ({
      name: s.name,
      type: 'line' as const,
      smooth: true,
      showSymbol: false,
      symbol: 'circle',
      symbolSize: 5,
      connectNulls: false,
      lineStyle: { width: 2, color: s.color },
      itemStyle: { color: s.color },
      data: sampleLatency(s.points).map((p) => ({
        value: [
          p.timestamp * 1000,
          p.packet_loss >= 100 ? null : p.latency_ms,
          p.jitter_ms,
          p.packet_loss
        ]
      }))
    }))
  }
}
