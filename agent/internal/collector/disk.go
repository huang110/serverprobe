package collector

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/disk"

	"github.com/huang110/serverprobe/agent/pkg/logger"
)

const (
	// bytesPerMB 字节到 MB 的换算系数
	bytesPerMB = 1024 * 1024

	// millisPerSecond 每秒的毫秒数，用于计算 IO 使用率
	millisPerSecond = 1000.0

	// percentBase 百分比基数
	percentBase = 100.0
)

// defaultExcludeFSTypes 默认排除的虚拟/伪文件系统。
// 这些文件系统不占用真实磁盘，统计它们没有意义，且部分无法访问。
var defaultExcludeFSTypes = []string{
	"tmpfs",
	"devtmpfs",
	"squashfs",
	"overlay",
	"aufs",
	"proc",
	"sysfs",
	"cgroup",
	"cgroup2",
	"ramfs",
	"devpts",
	"mqueue",
	"hugetlbfs",
	"debugfs",
	"tracefs",
	"securityfs",
	"pstore",
	"configfs",
	"binfmt_misc",
	"autofs",
	"nsfs",
	"bpf",
	"efivarfs",
	"fusectl",
	"selinuxfs",
	"rpc_pipefs",
	"nfsd",
	"fuse.gvfsd-fuse",
	"fuse.snapfuse",
	"fuse.portal",
	"fuse.lxcfs",
}

// DiskCollector 磁盘指标采集器。
// IO 速率、IOPS 和 IO 使用率依赖两次采样，因此采集器需要保存上一次的
// IO 计数器快照。Collect 方法通过互斥锁保证并发安全。
type DiskCollector struct {
	mu             sync.Mutex
	mountPoints    []string
	excludeFSTypes map[string]struct{}
	lastIOStats    map[string]disk.IOCountersStat
	lastIOTime     time.Time
}

// DiskCollectorOption 磁盘采集器可选配置
type DiskCollectorOption func(*DiskCollector)

// WithDiskMountPoints 设置需要监控的挂载点。
// 支持精确匹配和 filepath.Match 通配符（如 /data*）。
// 不设置或传入空列表时监控所有分区。
func WithDiskMountPoints(mountPoints []string) DiskCollectorOption {
	return func(c *DiskCollector) {
		c.mountPoints = make([]string, 0, len(mountPoints))
		for _, mountPoint := range mountPoints {
			if trimmed := strings.TrimSpace(mountPoint); trimmed != "" {
				c.mountPoints = append(c.mountPoints, trimmed)
			}
		}
	}
}

// WithExcludedFSTypes 在默认虚拟文件系统列表之外追加需要排除的文件系统类型。
func WithExcludedFSTypes(fsTypes []string) DiskCollectorOption {
	return func(c *DiskCollector) {
		for _, fsType := range fsTypes {
			normalized := strings.ToLower(strings.TrimSpace(fsType))
			if normalized != "" {
				c.excludeFSTypes[normalized] = struct{}{}
			}
		}
	}
}

// NewDiskCollector 创建磁盘采集器
func NewDiskCollector(opts ...DiskCollectorOption) *DiskCollector {
	c := &DiskCollector{
		excludeFSTypes: make(map[string]struct{}, len(defaultExcludeFSTypes)),
		lastIOStats:    make(map[string]disk.IOCountersStat),
	}

	// 默认排除虚拟文件系统
	for _, fsType := range defaultExcludeFSTypes {
		c.excludeFSTypes[fsType] = struct{}{}
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// Collect 采集所有匹配分区的磁盘指标。
// 单个分区无法访问时会跳过并记录警告，不会中断整个采集过程。
func (c *DiskCollector) Collect() ([]DiskMetrics, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, fmt.Errorf("failed to list disk partitions: %w", err)
	}

	now := time.Now()
	var sampleDuration float64
	if !c.lastIOTime.IsZero() {
		sampleDuration = now.Sub(c.lastIOTime).Seconds()
	}

	// 一次采集全部磁盘 IO 计数器，避免每个分区重复读取 /proc/diskstats
	ioCounters, err := disk.IOCounters()
	if err != nil {
		// IO 统计失败不应导致使用率数据也无法上报，重置快照避免下次算出错误速率
		logger.Warnf("Failed to get disk IO counters: %v", err)
		c.resetIOSample()
		ioCounters = nil
	}

	ioStatsByKey := make(map[string]disk.IOCountersStat, len(ioCounters))
	for _, stat := range ioCounters {
		ioStatsByKey[stat.Name] = stat
	}

	nextIOStats := make(map[string]disk.IOCountersStat, len(ioCounters))
	seenMounts := make(map[string]struct{}, len(partitions))
	metrics := make([]DiskMetrics, 0, len(partitions))

	for _, partition := range partitions {
		if c.isExcludedFS(partition.Fstype) {
			continue
		}
		if !c.shouldMonitorMount(partition.Mountpoint) {
			continue
		}

		// 同一挂载点可能因 bind mount 等原因重复出现，去重避免重复上报
		mountKey := normalizeMountPoint(partition.Mountpoint)
		if _, seen := seenMounts[mountKey]; seen {
			continue
		}
		seenMounts[mountKey] = struct{}{}

		usage, err := disk.Usage(partition.Mountpoint)
		if err != nil {
			logger.Warnf("Failed to get disk usage for %s: %v", partition.Mountpoint, err)
			continue
		}

		metric := DiskMetrics{
			Device:            partition.Device,
			MountPoint:        partition.Mountpoint,
			FsType:            partition.Fstype,
			Total:             int64(usage.Total / bytesPerMB),
			Used:              int64(usage.Used / bytesPerMB),
			Free:              int64(usage.Free / bytesPerMB),
			UsagePercent:      clampPercent(usage.UsedPercent),
			InodeTotal:        int64(usage.InodesTotal),
			InodeUsed:         int64(usage.InodesUsed),
			InodeUsagePercent: clampPercent(usage.InodesUsedPercent),
		}

		if stat, ok := findIOStat(partition.Device, ioStatsByKey); ok {
			// 保存本次快照供下一次采集计算差值
			nextIOStats[stat.Name] = stat

			// 第一次采集或采样间隔无效时，速率保持为 0
			if sampleDuration > 0 {
				if previous, exists := c.lastIOStats[stat.Name]; exists {
					rates := calculateDiskIO(previous, stat, sampleDuration)
					metric.ReadBytesPerSec = rates.ReadBytesPerSec
					metric.WriteBytesPerSec = rates.WriteBytesPerSec
					metric.ReadIOPS = rates.ReadIOPS
					metric.WriteIOPS = rates.WriteIOPS
					metric.IOUtilPercent = rates.IOUtilPercent
				}
			}
		}

		metrics = append(metrics, metric)
	}

	c.lastIOStats = nextIOStats
	c.lastIOTime = now

	return metrics, nil
}

// resetIOSample 清空 IO 快照，下一次采集将重新建立基准
func (c *DiskCollector) resetIOSample() {
	c.lastIOStats = make(map[string]disk.IOCountersStat)
	c.lastIOTime = time.Time{}
}

// isExcludedFS 判断文件系统类型是否在排除列表中
func (c *DiskCollector) isExcludedFS(fsType string) bool {
	_, excluded := c.excludeFSTypes[strings.ToLower(strings.TrimSpace(fsType))]
	return excluded
}

// shouldMonitorMount 判断挂载点是否匹配配置的过滤规则
func (c *DiskCollector) shouldMonitorMount(mountPoint string) bool {
	if len(c.mountPoints) == 0 {
		return true
	}

	for _, pattern := range c.mountPoints {
		if matchMountPattern(pattern, mountPoint) {
			return true
		}
	}
	return false
}

// matchMountPattern 匹配挂载点，支持精确路径和通配符
func matchMountPattern(pattern, mountPoint string) bool {
	pattern = normalizeMountPoint(pattern)
	mountPoint = normalizeMountPoint(mountPoint)
	if pattern == "" || mountPoint == "" {
		return false
	}

	// Windows 路径不区分大小写
	if runtime.GOOS == "windows" {
		pattern = strings.ToLower(pattern)
		mountPoint = strings.ToLower(mountPoint)
	}

	if pattern == mountPoint {
		return true
	}

	matched, err := filepath.Match(pattern, mountPoint)
	if err != nil {
		// 非法通配符视为不匹配，避免影响其他挂载点
		return false
	}
	return matched
}

// normalizeMountPoint 规范化挂载点路径
func normalizeMountPoint(mountPoint string) string {
	mountPoint = strings.TrimSpace(mountPoint)
	if mountPoint == "" {
		return ""
	}
	return filepath.Clean(mountPoint)
}

// findIOStat 根据分区设备名查找对应的 IO 计数器。
// Linux 下分区设备名通常为 /dev/sda1，而 /proc/diskstats 中的名称为 sda1；
// LVM 等设备还需要解析符号链接（如 /dev/mapper/vg-root -> /dev/dm-0）。
func findIOStat(device string, stats map[string]disk.IOCountersStat) (disk.IOCountersStat, bool) {
	candidates := ioCounterCandidates(device)
	for _, candidate := range candidates {
		if stat, ok := stats[candidate]; ok {
			return stat, true
		}
	}

	// 某些平台（如 Windows）使用 PhysicalDriveN 作为键，需要通过 Label 兜底匹配
	for _, stat := range stats {
		for _, candidate := range candidates {
			if strings.EqualFold(stat.Name, candidate) || strings.EqualFold(stat.Label, candidate) {
				return stat, true
			}
		}
	}

	return disk.IOCountersStat{}, false
}

// ioCounterCandidates 生成设备名可能对应的 IO 计数器名称列表
func ioCounterCandidates(device string) []string {
	device = strings.TrimSpace(device)
	if device == "" {
		return nil
	}

	candidates := make([]string, 0, 2)
	addCandidate := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		for _, existing := range candidates {
			if existing == name {
				return
			}
		}
		candidates = append(candidates, name)
	}

	addCandidate(filepath.Base(device))

	// 解析软链接，LVM/dm 设备的真实名称通常为 dm-N
	if resolved, err := filepath.EvalSymlinks(device); err == nil {
		addCandidate(filepath.Base(resolved))
	}

	return candidates
}

// diskIORates IO 速率计算结果
type diskIORates struct {
	ReadBytesPerSec  float64
	WriteBytesPerSec float64
	ReadIOPS         float64
	WriteIOPS        float64
	IOUtilPercent    float64
}

// calculateDiskIO 根据两次计数器快照计算 IO 速率。
// duration 为两次采样的间隔（秒），必须大于 0。
// 计数器回绕或重置时对应增量按 0 处理。
func calculateDiskIO(previous, current disk.IOCountersStat, duration float64) diskIORates {
	if duration <= 0 {
		return diskIORates{}
	}

	readBytes := counterDelta(current.ReadBytes, previous.ReadBytes)
	writeBytes := counterDelta(current.WriteBytes, previous.WriteBytes)
	readCount := counterDelta(current.ReadCount, previous.ReadCount)
	writeCount := counterDelta(current.WriteCount, previous.WriteCount)
	ioTimeMillis := counterDelta(current.IoTime, previous.IoTime)

	// IoTime 单位为毫秒；utilization = IO 时间 / 采样总时间 * 100
	ioUtil := float64(ioTimeMillis) / (duration * millisPerSecond) * percentBase

	return diskIORates{
		ReadBytesPerSec:  float64(readBytes) / duration,
		WriteBytesPerSec: float64(writeBytes) / duration,
		ReadIOPS:         float64(readCount) / duration,
		WriteIOPS:        float64(writeCount) / duration,
		IOUtilPercent:    clampPercent(ioUtil),
	}
}

// counterDelta 计算计数器增量，检测到重置时返回 0
func counterDelta(current, previous uint64) uint64 {
	if current < previous {
		return 0
	}
	return current - previous
}

// clampPercent 将百分比限制在 [0, 100] 区间
func clampPercent(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > percentBase {
		return percentBase
	}
	return value
}

// DiskMetrics 磁盘指标
type DiskMetrics struct {
	Device            string  // 设备名（如 /dev/sda1）
	MountPoint        string  // 挂载点（如 /）
	FsType            string  // 文件系统类型（ext4/xfs）
	Total             int64   // 总容量 MB
	Used              int64   // 已用 MB
	Free              int64   // 空闲 MB
	UsagePercent      float64 // 使用率 %
	InodeTotal        int64   // Inode 总数
	InodeUsed         int64   // Inode 已用
	InodeUsagePercent float64 // Inode 使用率 %
	ReadBytesPerSec   float64 // 读取速率 Bytes/s
	WriteBytesPerSec  float64 // 写入速率 Bytes/s
	ReadIOPS          float64 // 读 IOPS
	WriteIOPS         float64 // 写 IOPS
	IOUtilPercent     float64 // IO 使用率 %
}
