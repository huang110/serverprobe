package collector

import (
	"math"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/huang110/serverprobe/agent/internal/config"
)

const (
	// defaultLatencyCheckInterval 两次真实探测之间的最小间隔
	defaultLatencyCheckInterval = 15 * time.Second

	// defaultLatencyProbeCount 单次探测每个目标发起的 TCP 连接次数
	defaultLatencyProbeCount = 3

	// defaultLatencyProbeInterval 同一目标连续探测之间的等待时间
	defaultLatencyProbeInterval = 50 * time.Millisecond

	// defaultLatencyDialTimeout 单次 TCP 连接超时
	defaultLatencyDialTimeout = 1500 * time.Millisecond

	// defaultLatencyPort 未指定端口时使用的默认端口
	defaultLatencyPort = "443"

	// dnsLatencyPort DNS 服务器默认使用 53 端口
	dnsLatencyPort = "53"
)

// latencyDialFunc 可注入的 TCP 拨号函数，便于单元测试
type latencyDialFunc func(network, address string, timeout time.Duration) (net.Conn, error)

// LatencyResult 单个 TCP Ping 目标的探测结果
type LatencyResult struct {
	TargetName string
	Target     string
	LatencyMs  float64
	JitterMs   float64
	PacketLoss float64
}

// LatencyOption LatencyCollector 可选配置
type LatencyOption func(*LatencyCollector)

// WithLatencyCheckInterval 设置真实探测的最小间隔
func WithLatencyCheckInterval(interval time.Duration) LatencyOption {
	return func(c *LatencyCollector) {
		if interval > 0 {
			c.checkInterval = interval
		}
	}
}

// WithLatencyProbeCount 设置每个目标的探测次数
func WithLatencyProbeCount(count int) LatencyOption {
	return func(c *LatencyCollector) {
		if count > 0 {
			c.probeCount = count
		}
	}
}

// WithLatencyProbeInterval 设置同一目标连续探测的间隔
func WithLatencyProbeInterval(interval time.Duration) LatencyOption {
	return func(c *LatencyCollector) {
		if interval >= 0 {
			c.probeInterval = interval
		}
	}
}

// WithLatencyDialTimeout 设置单次 TCP 连接超时
func WithLatencyDialTimeout(timeout time.Duration) LatencyOption {
	return func(c *LatencyCollector) {
		if timeout > 0 {
			c.dialTimeout = timeout
		}
	}
}

// WithLatencyDialer 注入自定义拨号函数（测试用）
func WithLatencyDialer(dial latencyDialFunc) LatencyOption {
	return func(c *LatencyCollector) {
		if dial != nil {
			c.dial = dial
		}
	}
}

// WithLatencyClock 注入自定义时钟（测试用）
func WithLatencyClock(now func() time.Time) LatencyOption {
	return func(c *LatencyCollector) {
		if now != nil {
			c.now = now
		}
	}
}

// LatencyCollector 多目标 TCP Ping 延迟采集器。
// 内部缓存最近一次探测结果，避免每次心跳都发起大量 TCP 连接。
type LatencyCollector struct {
	targets       []config.LatencyTarget
	checkInterval time.Duration
	probeCount    int
	probeInterval time.Duration
	dialTimeout   time.Duration
	dial          latencyDialFunc
	now           func() time.Time

	mu        sync.Mutex
	cached    []LatencyResult
	lastCheck time.Time

	// collectMu 保证并发调用时同一时刻只有一次真实探测
	collectMu sync.Mutex
}

// NewLatencyCollector 创建 TCP Ping 采集器
func NewLatencyCollector(targets []config.LatencyTarget, options ...LatencyOption) *LatencyCollector {
	collector := &LatencyCollector{
		targets:       append([]config.LatencyTarget(nil), targets...),
		checkInterval: defaultLatencyCheckInterval,
		probeCount:    defaultLatencyProbeCount,
		probeInterval: defaultLatencyProbeInterval,
		dialTimeout:   defaultLatencyDialTimeout,
		dial:          net.DialTimeout,
		now:           time.Now,
	}
	for _, option := range options {
		option(collector)
	}
	if collector.now == nil {
		collector.now = time.Now
	}
	return collector
}

// Collect 返回最近一次探测结果。
// 距离上次探测不足 checkInterval 时直接返回缓存，否则重新探测。
func (c *LatencyCollector) Collect() []LatencyResult {
	if c == nil {
		return nil
	}

	c.mu.Lock()
	if !c.lastCheck.IsZero() && c.now().Sub(c.lastCheck) < c.checkInterval {
		results := cloneLatencyResults(c.cached)
		c.mu.Unlock()
		return results
	}
	c.mu.Unlock()

	c.collectMu.Lock()
	defer c.collectMu.Unlock()

	// 双重检查，避免并发调用重复探测
	c.mu.Lock()
	if !c.lastCheck.IsZero() && c.now().Sub(c.lastCheck) < c.checkInterval {
		results := cloneLatencyResults(c.cached)
		c.mu.Unlock()
		return results
	}
	c.mu.Unlock()

	results := c.probeAll()

	c.mu.Lock()
	c.cached = results
	c.lastCheck = c.now()
	c.mu.Unlock()

	return cloneLatencyResults(results)
}

// LastCheckTime 返回最近一次真实探测的时间戳（Unix 秒），未探测时返回 0
func (c *LatencyCollector) LastCheckTime() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastCheck.IsZero() {
		return 0
	}
	return c.lastCheck.Unix()
}

// probeAll 并发探测全部目标
func (c *LatencyCollector) probeAll() []LatencyResult {
	if len(c.targets) == 0 {
		return nil
	}

	results := make([]LatencyResult, len(c.targets))
	var wg sync.WaitGroup

	for index, target := range c.targets {
		wg.Add(1)
		go func(index int, target config.LatencyTarget) {
			defer wg.Done()
			results[index] = c.probeTarget(target)
		}(index, target)
	}

	wg.Wait()
	return results
}

// probeTarget 对单个目标连续探测多次并统计延迟、抖动和丢包率
func (c *LatencyCollector) probeTarget(target config.LatencyTarget) LatencyResult {
	address := normalizeLatencyAddress(target.Address)
	name := strings.TrimSpace(target.Name)
	if name == "" {
		name = address
	}

	result := LatencyResult{
		TargetName: name,
		Target:     address,
		PacketLoss: 100,
	}
	if address == "" {
		return result
	}

	samples := make([]float64, 0, c.probeCount)
	failed := 0

	for attempt := 0; attempt < c.probeCount; attempt++ {
		if attempt > 0 && c.probeInterval > 0 {
			time.Sleep(c.probeInterval)
		}

		start := c.now()
		conn, err := c.dial("tcp", address, c.dialTimeout)
		elapsed := c.now().Sub(start)
		if err != nil {
			failed++
			continue
		}
		if conn != nil {
			_ = conn.Close()
		}
		samples = append(samples, float64(elapsed)/float64(time.Millisecond))
	}

	total := c.probeCount
	if total <= 0 {
		total = defaultLatencyProbeCount
	}
	result.PacketLoss = roundLatency(float64(failed) / float64(total) * 100)

	if len(samples) == 0 {
		result.LatencyMs = 0
		result.JitterMs = 0
		return result
	}

	sum := 0.0
	minimum := samples[0]
	maximum := samples[0]
	for _, sample := range samples {
		sum += sample
		if sample < minimum {
			minimum = sample
		}
		if sample > maximum {
			maximum = sample
		}
	}

	result.LatencyMs = roundLatency(sum / float64(len(samples)))
	result.JitterMs = roundLatency(maximum - minimum)
	return result
}

// normalizeLatencyAddress 规范化目标地址。
// 未指定端口时默认补全 443；8.8.8.8 / 114.114.114.114 默认使用 53 端口。
func normalizeLatencyAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	host, port, err := net.SplitHostPort(address)
	if err == nil {
		if strings.TrimSpace(port) != "" {
			return address
		}
		return net.JoinHostPort(host, defaultPortForHost(host))
	}

	host = strings.Trim(address, "[]")
	if host == "" {
		return ""
	}
	return net.JoinHostPort(host, defaultPortForHost(host))
}

// defaultPortForHost 根据目标主机选择默认端口
func defaultPortForHost(host string) string {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "8.8.8.8", "114.114.114.114":
		return dnsLatencyPort
	default:
		return defaultLatencyPort
	}
}

// cloneLatencyResults 复制结果，避免调用方修改内部缓存
func cloneLatencyResults(results []LatencyResult) []LatencyResult {
	if len(results) == 0 {
		return nil
	}
	return append([]LatencyResult(nil), results...)
}

// roundLatency 保留两位小数
func roundLatency(value float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return math.Round(value*100) / 100
}
