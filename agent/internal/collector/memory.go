package collector

import (
	"github.com/shirou/gopsutil/v3/mem"
)

// MemoryCollector 内存指标采集器
type MemoryCollector struct{}

// NewMemoryCollector 创建内存采集器
func NewMemoryCollector() *MemoryCollector {
	return &MemoryCollector{}
}

// Collect 采集内存指标
func (m *MemoryCollector) Collect() (*MemoryMetrics, error) {
	vmStat, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	swapStat, err := mem.SwapMemory()
	if err != nil {
		return nil, err
	}

	metrics := &MemoryMetrics{
		Total:            int64(vmStat.Total / 1024 / 1024), // MB
		Used:             int64(vmStat.Used / 1024 / 1024),
		Free:             int64(vmStat.Free / 1024 / 1024),
		Available:        int64(vmStat.Available / 1024 / 1024),
		UsagePercent:     vmStat.UsedPercent,
		SwapTotal:        int64(swapStat.Total / 1024 / 1024),
		SwapUsed:         int64(swapStat.Used / 1024 / 1024),
		SwapFree:         int64(swapStat.Free / 1024 / 1024),
		SwapUsagePercent: swapStat.UsedPercent,
		Buffers:          int64(vmStat.Buffers / 1024 / 1024),
		Cached:           int64(vmStat.Cached / 1024 / 1024),
		Shared:           int64(vmStat.Shared / 1024 / 1024),
	}

	return metrics, nil
}

// MemoryMetrics 内存指标
type MemoryMetrics struct {
	Total            int64
	Used             int64
	Free             int64
	Available        int64
	UsagePercent     float64
	SwapTotal        int64
	SwapUsed         int64
	SwapFree         int64
	SwapUsagePercent float64
	Buffers          int64
	Cached           int64
	Shared           int64
}
