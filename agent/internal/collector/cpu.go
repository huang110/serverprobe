package collector

import (
	"runtime"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

const (
	// cpuSampleInterval CPU 总体使用率采样窗口
	cpuSampleInterval = 500 * time.Millisecond
)

// CPUCollector CPU 指标采集器
type CPUCollector struct {
	mu           sync.Mutex
	lastCPUTimes []cpu.TimesStat
}

// NewCPUCollector 创建 CPU 采集器
func NewCPUCollector() *CPUCollector {
	return &CPUCollector{}
}

// Collect 采集 CPU 指标。
// 总体使用率采样 500ms；各状态百分比通过两次 cpu.Times 的差值计算。
func (c *CPUCollector) Collect() (*CPUMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	metrics := &CPUMetrics{}

	// 获取总体 CPU 使用率
	percent, err := cpu.Percent(cpuSampleInterval, false)
	if err != nil {
		return nil, err
	}
	if len(percent) > 0 {
		metrics.UsagePercent = percent[0]
	}

	// 获取累计 CPU 时间，使用上次快照计算 delta
	times, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}
	if len(times) > 0 {
		var previous *cpu.TimesStat
		if len(c.lastCPUTimes) > 0 {
			previous = &c.lastCPUTimes[0]
		}

		metrics.UserPercent, metrics.SystemPercent, metrics.IdlePercent,
			metrics.IowaitPercent, metrics.StealPercent = calculateCPUPercentages(times[0], previous)

		c.lastCPUTimes = times
	}

	return metrics, nil
}

// calculateCPUPercentages 计算 CPU 各状态百分比。
// previous 为 nil 时使用累计值兜底（首次采样）。
func calculateCPUPercentages(current cpu.TimesStat, previous *cpu.TimesStat) (user, system, idle, iowait, steal float64) {
	userValue := current.User
	systemValue := current.System
	idleValue := current.Idle
	iowaitValue := current.Iowait
	stealValue := current.Steal

	if previous != nil {
		userValue = counterDeltaFloat(current.User, previous.User)
		systemValue = counterDeltaFloat(current.System, previous.System)
		idleValue = counterDeltaFloat(current.Idle, previous.Idle)
		iowaitValue = counterDeltaFloat(current.Iowait, previous.Iowait)
		stealValue = counterDeltaFloat(current.Steal, previous.Steal)

		// 总时间同样按 delta 计算，Nice/Irq/Softirq 不能遗漏
		total := counterDeltaFloat(current.User, previous.User) +
			counterDeltaFloat(current.Nice, previous.Nice) +
			counterDeltaFloat(current.System, previous.System) +
			counterDeltaFloat(current.Idle, previous.Idle) +
			counterDeltaFloat(current.Iowait, previous.Iowait) +
			counterDeltaFloat(current.Irq, previous.Irq) +
			counterDeltaFloat(current.Softirq, previous.Softirq) +
			counterDeltaFloat(current.Steal, previous.Steal)
		if total > 0 {
			return userValue / total * 100,
				systemValue / total * 100,
				idleValue / total * 100,
				iowaitValue / total * 100,
				stealValue / total * 100
		}
	}

	total := current.User + current.Nice + current.System + current.Idle +
		current.Iowait + current.Irq + current.Softirq + current.Steal
	if total <= 0 {
		return 0, 0, 0, 0, 0
	}

	return userValue / total * 100,
		systemValue / total * 100,
		idleValue / total * 100,
		iowaitValue / total * 100,
		stealValue / total * 100
}

// counterDeltaFloat 计算浮点计数器增量，检测到重置时返回 0
func counterDeltaFloat(current, previous float64) float64 {
	if current < previous {
		return 0
	}
	return current - previous
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
