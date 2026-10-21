package collector

import (
	"runtime"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

// CPUCollector CPU 指标采集器
type CPUCollector struct {
	lastCPUTimes []cpu.TimesStat
	lastTime     time.Time
}

// NewCPUCollector 创建 CPU 采集器
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

// Collect 采集 CPU 指标
func (c *CPUCollector) Collect() (*CPUMetrics, error) {
	metrics := &CPUMetrics{}

	// 获取总体 CPU 使用率
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		return nil, err
	}
	if len(percent) > 0 {
		metrics.UsagePercent = percent[0]
	}

	// 获取详细的 CPU 时间统计
	times, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}

	if len(times) > 0 {
		total := times[0].User + times[0].System + times[0].Idle + times[0].Iowait + times[0].Steal
		if total > 0 {
			metrics.UserPercent = (times[0].User / total) * 100
			metrics.SystemPercent = (times[0].System / total) * 100
			metrics.IdlePercent = (times[0].Idle / total) * 100
			metrics.IowaitPercent = (times[0].Iowait / total) * 100
			metrics.StealPercent = (times[0].Steal / total) * 100
		}
	}

	// 获取每个核心的使用率
	perCPU, err := cpu.Percent(time.Second, true)
	if err == nil {
		metrics.PerCoreUsage = perCPU
	}

	return metrics, nil
}

// CPUMetrics CPU 指标
type CPUMetrics struct {
	UsagePercent  float64
	UserPercent   float64
	SystemPercent float64
	IdlePercent   float64
	IowaitPercent float64
	StealPercent  float64
	PerCoreUsage  []float64
}

// GetCPUCores 获取 CPU 核心数
func GetCPUCores() (int32, error) {
	counts, err := cpu.Counts(true)
	if err != nil {
		// Fallback to runtime
		return int32(runtime.NumCPU()), nil
	}
	return int32(counts), nil
}
