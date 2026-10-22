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
