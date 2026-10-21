package collector

import (
	"testing"

	gnet "github.com/shirou/gopsutil/v3/net"
)

// TestCountConnectionStatuses 验证连接状态统计和大小写归一化
func TestCountConnectionStatuses(t *testing.T) {
	conns := []gnet.ConnectionStat{
		{Status: "ESTABLISHED"},
		{Status: "established"},
		{Status: " ESTABLISHED "},
		{Status: "TIME_WAIT"},
		{Status: "CLOSE_WAIT"},
		{Status: "LISTEN"},
		{Status: ""},
	}

	stats := countConnectionStatuses(conns)

	expected := map[string]int32{
		"ESTABLISHED": 3,
		"TIME_WAIT":   1,
		"CLOSE_WAIT":  1,
		"LISTEN":      1,
	}

	for status, want := range expected {
		if got := stats[status]; got != want {
			t.Errorf("status %s = %d, want %d", status, got, want)
		}
	}

	if _, exists := stats[""]; exists {
		t.Error("empty status should be ignored")
	}
}

// TestSystemCollectorCollect 对真实系统执行采集，验证返回值的基本约束
func TestSystemCollectorCollect(t *testing.T) {
	c := NewSystemCollector()

	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}
	if metrics == nil {
		t.Fatal("Collect() returned nil metrics")
	}

	if metrics.Load1 < 0 || metrics.Load5 < 0 || metrics.Load15 < 0 {
		t.Errorf("load average should not be negative: %+v", metrics)
	}
	if metrics.Uptime < 0 {
		t.Errorf("uptime should not be negative: %d", metrics.Uptime)
	}
	if metrics.ProcessCount <= 0 {
		t.Errorf("process count should be positive: %d", metrics.ProcessCount)
	}
	if metrics.TcpEstablished < 0 || metrics.TcpTimeWait < 0 || metrics.TcpCloseWait < 0 {
		t.Errorf("TCP counters should not be negative: %+v", metrics)
	}
	if metrics.UdpConnections < 0 {
		t.Errorf("UDP connections should not be negative: %d", metrics.UdpConnections)
	}
}

// TestGetTCPStats 验证 TCP 连接统计接口可正常调用
func TestGetTCPStats(t *testing.T) {
	stats, err := getTCPStats()
	if err != nil {
		// 某些平台/权限下可能无法读取连接信息，此时采集器会降级处理
		t.Skipf("TCP connections are not available in this environment: %v", err)
	}
	if stats == nil {
		t.Error("getTCPStats() should return a non-nil map")
	}
}
