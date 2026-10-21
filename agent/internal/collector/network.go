package collector

import (
	"fmt"
	"math"
	"net"
	"path"
	"strings"
	"sync"
	"time"

	gnet "github.com/shirou/gopsutil/v3/net"

	"github.com/huang110/serverprobe/agent/pkg/logger"
)

// NetworkCollector 网络指标采集器。
// 实时速率依赖两次 IOCounters 采样，因此需要保存上一次的快照。
// Collect 方法通过互斥锁保证并发安全。
type NetworkCollector struct {
	mu            sync.Mutex
	interfaces    []string
	lastStats     map[string]gnet.IOCountersStat
	lastTimestamp time.Time
}

// NetworkCollectorOption 网络采集器可选配置
type NetworkCollectorOption func(*NetworkCollector)

// WithNetworkInterfaces 设置需要监控的网卡名称。
// 支持精确匹配和 path.Match 通配符（如 eth*、ens*）。
// 不设置或传入空列表时监控除回环接口外的所有网卡。
func WithNetworkInterfaces(interfaces []string) NetworkCollectorOption {
	return func(c *NetworkCollector) {
		c.interfaces = make([]string, 0, len(interfaces))
		for _, iface := range interfaces {
			if trimmed := strings.TrimSpace(iface); trimmed != "" {
				c.interfaces = append(c.interfaces, trimmed)
			}
		}
	}
}

// NewNetworkCollector 创建网络采集器
func NewNetworkCollector(opts ...NetworkCollectorOption) *NetworkCollector {
	c := &NetworkCollector{
		lastStats: make(map[string]gnet.IOCountersStat),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Collect 采集网络接口指标。
// 第一次采集时没有历史快照，速率字段为 0。
func (n *NetworkCollector) Collect() ([]NetworkMetrics, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	// 获取每个网卡的累计流量和包统计
	stats, err := gnet.IOCounters(true)
	if err != nil {
		return nil, fmt.Errorf("failed to get network IO counters: %w", err)
	}

	// 获取网卡的 IP、MAC 和回环标记。元数据失败时仍继续上报流量数据
	interfaces, err := gnet.Interfaces()
	if err != nil {
		logger.Warnf("Failed to get network interfaces: %v", err)
		interfaces = nil
	}
	interfaceMap := make(map[string]gnet.InterfaceStat, len(interfaces))
	for _, iface := range interfaces {
		interfaceMap[iface.Name] = iface
	}

	now := time.Now()
	var sampleDuration float64
	if !n.lastTimestamp.IsZero() {
		sampleDuration = now.Sub(n.lastTimestamp).Seconds()
	}

	nextStats := make(map[string]gnet.IOCountersStat, len(stats))
	metrics := make([]NetworkMetrics, 0, len(stats))

	for _, stat := range stats {
		iface, hasMetadata := interfaceMap[stat.Name]
		if isLoopbackInterface(stat.Name, iface.Flags) {
			continue
		}
		if !n.shouldMonitorInterface(stat.Name) {
			continue
		}

		metric := NetworkMetrics{
			Interface:    stat.Name,
			RxBytesTotal: int64(stat.BytesRecv),
			TxBytesTotal: int64(stat.BytesSent),
			RxPackets:    int64(stat.PacketsRecv),
			TxPackets:    int64(stat.PacketsSent),
			RxErrors:     int64(stat.Errin),
			TxErrors:     int64(stat.Errout),
			RxDropped:    int64(stat.Dropin),
			TxDropped:    int64(stat.Dropout),
		}

		if hasMetadata {
			metric.IPAddress = selectIPAddress(iface.Addrs)
			metric.MACAddress = strings.TrimSpace(iface.HardwareAddr)
		}

		// 第一次采集或采样间隔无效时，速率保持为 0
		if sampleDuration > 0 {
			if previous, exists := n.lastStats[stat.Name]; exists {
				metric.RxBytesPerSec, metric.TxBytesPerSec = calculateNetworkRates(previous, stat, sampleDuration)
			}
		}

		nextStats[stat.Name] = stat
		metrics = append(metrics, metric)
	}

	n.lastStats = nextStats
	n.lastTimestamp = now

	return metrics, nil
}

// shouldMonitorInterface 判断网卡是否匹配配置的过滤规则
func (n *NetworkCollector) shouldMonitorInterface(interfaceName string) bool {
	if len(n.interfaces) == 0 {
		return true
	}

	for _, pattern := range n.interfaces {
		if matchInterfacePattern(pattern, interfaceName) {
			return true
		}
	}
	return false
}

// matchInterfacePattern 匹配网卡名称，支持精确名称和通配符
func matchInterfacePattern(pattern, interfaceName string) bool {
	pattern = strings.TrimSpace(pattern)
	interfaceName = strings.TrimSpace(interfaceName)
	if pattern == "" || interfaceName == "" {
		return false
	}

	if pattern == interfaceName {
		return true
	}

	matched, err := path.Match(pattern, interfaceName)
	if err != nil {
		// 非法通配符视为不匹配，避免影响其他网卡
		return false
	}
	return matched
}

// isLoopbackInterface 判断是否为回环接口。
// 同时依据网卡名称和系统返回的 flags，兼容 Linux（lo）、macOS（lo0）
// 和 Windows（Loopback Pseudo-Interface 1）。
func isLoopbackInterface(interfaceName string, flags []string) bool {
	lowerName := strings.ToLower(strings.TrimSpace(interfaceName))
	if lowerName == "lo" || strings.HasPrefix(lowerName, "lo0") || strings.Contains(lowerName, "loopback") {
		return true
	}

	for _, flag := range flags {
		if strings.EqualFold(strings.TrimSpace(flag), "loopback") {
			return true
		}
	}
	return false
}

// selectIPAddress 从网卡地址列表中选择用于上报的 IP。
// 优先级：非回环、非链路本地的 IPv4 > IPv4 > 非链路本地 IPv6 > IPv6。
func selectIPAddress(addrs gnet.InterfaceAddrList) string {
	var fallbackIPv4 string
	var preferredIPv6 string
	var fallbackIPv6 string

	for _, addr := range addrs {
		ip, _, err := net.ParseCIDR(strings.TrimSpace(addr.Addr))
		if err != nil {
			continue
		}
		if ip.IsLoopback() {
			continue
		}

		if ip.To4() != nil {
			ipv4 := ip.String()
			if !ip.IsLinkLocalUnicast() {
				return ipv4
			}
			if fallbackIPv4 == "" {
				fallbackIPv4 = ipv4
			}
			continue
		}

		ipv6 := ip.String()
		if ip.IsLinkLocalUnicast() {
			if fallbackIPv6 == "" {
				fallbackIPv6 = ipv6
			}
			continue
		}
		if preferredIPv6 == "" {
			preferredIPv6 = ipv6
		}
	}

	if fallbackIPv4 != "" {
		return fallbackIPv4
	}
	if preferredIPv6 != "" {
		return preferredIPv6
	}
	return fallbackIPv6
}

// calculateNetworkRates 根据两次计数器快照计算实时上下行速率（Bytes/s）。
// duration 为两次采样的间隔（秒）。计数器重置时对应增量按 0 处理。
func calculateNetworkRates(previous, current gnet.IOCountersStat, duration float64) (int64, int64) {
	if duration <= 0 {
		return 0, 0
	}

	rxBytesPerSec := float64(counterDelta(current.BytesRecv, previous.BytesRecv)) / duration
	txBytesPerSec := float64(counterDelta(current.BytesSent, previous.BytesSent)) / duration

	return clampInt64(rxBytesPerSec), clampInt64(txBytesPerSec)
}

// clampInt64 将浮点速率安全转换为 int64，避免溢出
func clampInt64(value float64) int64 {
	if value <= 0 {
		return 0
	}
	if value >= math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(value)
}

// NetworkMetrics 网络指标
type NetworkMetrics struct {
	Interface     string // 网卡名称（eth0/ens33）
	IPAddress     string // IP 地址
	MACAddress    string // MAC 地址
	RxBytesPerSec int64  // 入站速率 Bytes/s
	TxBytesPerSec int64  // 出站速率 Bytes/s
	RxBytesTotal  int64  // 累计接收字节
	TxBytesTotal  int64  // 累计发送字节
	RxPackets     int64  // 接收包数
	TxPackets     int64  // 发送包数
	RxErrors      int64  // 接收错误
	TxErrors      int64  // 发送错误
	RxDropped     int64  // 接收丢包
	TxDropped     int64  // 发送丢包
}
