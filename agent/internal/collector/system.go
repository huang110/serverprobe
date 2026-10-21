package collector

import (
	"strings"

	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/load"
	gnet "github.com/shirou/gopsutil/v3/net"
	"github.com/shirou/gopsutil/v3/process"

	"github.com/huang110/serverprobe/agent/pkg/logger"
)

// SystemCollector 系统信息采集器。
// 该采集器无内部状态，可安全地被多个 goroutine 同时调用。
type SystemCollector struct{}

// NewSystemCollector 创建系统信息采集器
func NewSystemCollector() *SystemCollector {
	return &SystemCollector{}
}

// Collect 采集系统负载、运行时间、进程数和连接统计。
// 单项指标获取失败时记录 Warn 并继续采集其他指标，保证心跳仍能上报可用数据。
func (s *SystemCollector) Collect() (*SystemMetrics, error) {
	metrics := &SystemMetrics{}

	// 系统平均负载（Windows 首次采样可能为 0，需要预热）
	loadAvg, err := load.Avg()
	if err != nil {
		logger.Warnf("Failed to get system load average: %v", err)
	} else if loadAvg != nil {
		metrics.Load1 = loadAvg.Load1
		metrics.Load5 = loadAvg.Load5
		metrics.Load15 = loadAvg.Load15
	}

	// 系统运行时间（秒）
	uptime, err := host.Uptime()
	if err != nil {
		logger.Warnf("Failed to get system uptime: %v", err)
	} else {
		metrics.Uptime = int64(uptime)
	}

	// 进程总数
	pids, err := process.Pids()
	if err != nil {
		logger.Warnf("Failed to get process list: %v", err)
	} else {
		metrics.ProcessCount = int32(len(pids))
	}

	// TCP 连接状态统计
	tcpStats, err := getTCPStats()
	if err != nil {
		logger.Warnf("Failed to get TCP connections: %v", err)
	} else {
		metrics.TcpEstablished = tcpStats["ESTABLISHED"]
		metrics.TcpTimeWait = tcpStats["TIME_WAIT"]
		metrics.TcpCloseWait = tcpStats["CLOSE_WAIT"]
	}

	// UDP 连接数
	udpConnections, err := getUDPConnectionCount()
	if err != nil {
		logger.Warnf("Failed to get UDP connections: %v", err)
	} else {
		metrics.UdpConnections = udpConnections
	}

	return metrics, nil
}

// getTCPStats 获取 TCP 连接状态统计。
// 状态字符串统一转为大写，兼容不同平台的返回值差异。
func getTCPStats() (map[string]int32, error) {
	conns, err := gnet.Connections("tcp")
	if err != nil {
		return nil, err
	}

	return countConnectionStatuses(conns), nil
}

// countConnectionStatuses 统计各状态连接数
func countConnectionStatuses(conns []gnet.ConnectionStat) map[string]int32 {
	stats := make(map[string]int32)
	for _, conn := range conns {
		status := strings.ToUpper(strings.TrimSpace(conn.Status))
		if status == "" {
			continue
		}
		stats[status]++
	}
	return stats
}

// getUDPConnectionCount 获取 UDP 连接数
func getUDPConnectionCount() (int32, error) {
	conns, err := gnet.Connections("udp")
	if err != nil {
		return 0, err
	}
	return int32(len(conns)), nil
}

// SystemMetrics 系统指标
type SystemMetrics struct {
	Load1          float64 // 1 分钟负载
	Load5          float64 // 5 分钟负载
	Load15         float64 // 15 分钟负载
	Uptime         int64   // 系统运行时间（秒）
	ProcessCount   int32   // 进程数
	TcpEstablished int32   // TCP ESTABLISHED
	TcpTimeWait    int32   // TCP TIME_WAIT
	TcpCloseWait   int32   // TCP CLOSE_WAIT
	UdpConnections int32   // UDP 连接数
}
