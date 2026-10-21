/**
 * ECharts 通用配置工厂（深色主题）。
 */
import type { EChartsOption } from 'echarts'
import type { MetricPoint } from '@/types/metrics'

export interface SeriesDef {
  name: string
  color: string
  points: MetricPoint[]
}

/** 当数据点过多时做抽样，保证图表渲染性能 */
function sample(points: MetricPoint[], max = 1500): Array<[number, number]> {
  if (points.length <= max) {
    return points.map((p) => [p.timestamp * 1000, Number(p.value?.toFixed(2))])
  }
  const step = Math.ceil(points.length / max)
  const sampled: Array<[number, number]> = []
  for (let i = 0; i < points.length; i += step) {
    const p = points[i]
    sampled.push([p.timestamp * 1000, Number(p.value?.toFixed(2))])
  }
  return sampled
}

const AXIS_STYLE = {
  axisLine: { lineStyle: { color: '#2b313c' } },
  axisLabel: { color: '#8b94a3', fontSize: 11 },
  splitLine: { lineStyle: { color: 'rgba(43,49,60,0.55)', type: 'dashed' as const } }
}

function tooltipConfig(unit: string) {
  return {
    trigger: 'axis' as const,
    backgroundColor: 'rgba(18,22,30,0.95)',
    borderColor: '#2b313c',
    textStyle: { color: '#e5e9f0', fontSize: 12 },
    axisPointer: { type: 'line' as const, lineStyle: { color: '#3b82f6' } },
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
      textStyle: { color: '#e5e9f0', fontSize: 14, fontWeight: 600 }
    },
    tooltip: tooltipConfig(unit),
    legend: {
      data: seriesList.map((s) => s.name),
      top: 4,
      right: 8,
      textStyle: { color: '#8b94a3' },
      itemWidth: 14,
      itemHeight: 8
    },
    grid: { left: 48, right: 20, top: 48, bottom: 42 },
    xAxis: {
      type: 'time',
      ...AXIS_STYLE
    },
    yAxis: {
      type: 'value',
      min: percent ? 0 : undefined,
      max: percent ? 100 : undefined,
      ...AXIS_STYLE,
      axisLabel: {
        ...AXIS_STYLE.axisLabel,
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
      areaStyle: seriesList.length === 1
        ? {
            color: {
              type: 'linear' as const,
              x: 0, y: 0, x2: 0, y2: 1,
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
