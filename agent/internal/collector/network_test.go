package collector

import (
	"testing"

	gnet "github.com/shirou/gopsutil/v3/net"
)

// TestNetworkCollectorFiltering 验证网卡过滤规则
func TestNetworkCollectorFiltering(t *testing.T) {
	c := NewNetworkCollector(WithNetworkInterfaces([]string{" eth* ", "", "ens*"}))

	tests := []struct {
		interfaceName string
		want          bool
	}{
		{"eth0", true},
		{"eth1.100", true},
		{"ens33", true},
		{"en0", false},
		{"docker0", false},
		{"", false},
	}

	for _, test := range tests {
		if got := c.shouldMonitorInterface(test.interfaceName); got != test.want {
			t.Errorf("shouldMonitorInterface(%q) = %v, want %v", test.interfaceName, got, test.want)
		}
	}

	all := NewNetworkCollector()
	if !all.shouldMonitorInterface("anything0") {
		t.Error("collector without filters should monitor every interface")
	}
}

// TestIsLoopbackInterface 验证回环接口识别
func TestIsLoopbackInterface(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		want  bool
	}{
		{"lo", nil, true},
		{"lo0", nil, true},
		{"Loopback Pseudo-Interface 1", nil, true},
		{"ethernet_32768", []string{"up", "loopback"}, true},
		{"eth0", []string{"up", "broadcast"}, false},
		{"docker0", []string{"up"}, false},
	}

	for _, test := range tests {
		if got := isLoopbackInterface(test.name, test.flags); got != test.want {
			t.Errorf("isLoopbackInterface(%q, %v) = %v, want %v", test.name, test.flags, got, test.want)
		}
	}
}

// TestSelectIPAddress 验证 IP 地址优先级
func TestSelectIPAddress(t *testing.T) {
	tests := []struct {
		name  string
		addrs gnet.InterfaceAddrList
		want  string
	}{
		{
			name: "prefer IPv4",
			addrs: gnet.InterfaceAddrList{
				{Addr: "127.0.0.1/8"},
				{Addr: "fe80::1/64"},
				{Addr: "192.168.1.10/24"},
			},
			want: "192.168.1.10",
		},
		{
			name: "IPv6 only",
			addrs: gnet.InterfaceAddrList{
				{Addr: "2001:db8::1/64"},
			},
			want: "2001:db8::1",
		},
		{
			name: "fallback link local IPv4",
			addrs: gnet.InterfaceAddrList{
				{Addr: "169.254.10.20/16"},
			},
			want: "169.254.10.20",
		},
		{
			name: "loopback only",
			addrs: gnet.InterfaceAddrList{
				{Addr: "127.0.0.1/8"},
				{Addr: "::1/128"},
			},
			want: "",
		},
		{
			name: "ignore invalid address",
			addrs: gnet.InterfaceAddrList{
				{Addr: "not-an-ip"},
				{Addr: "10.0.0.5/24"},
			},
			want: "10.0.0.5",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := selectIPAddress(test.addrs); got != test.want {
				t.Errorf("selectIPAddress() = %q, want %q", got, test.want)
			}
		})
	}
}

// TestCalculateNetworkRates 验证两次采样之间的实时速率计算
func TestCalculateNetworkRates(t *testing.T) {
	previous := gnet.IOCountersStat{BytesRecv: 1000, BytesSent: 2000}
	current := gnet.IOCountersStat{BytesRecv: 3000, BytesSent: 6000}

	rx, tx := calculateNetworkRates(previous, current, 2)
	if rx != 1000 {
		t.Errorf("RxBytesPerSec = %d, want 1000", rx)
	}
	if tx != 2000 {
		t.Errorf("TxBytesPerSec = %d, want 2000", tx)
	}

	// 计数器重置时速率应为 0
	resetRx, resetTx := calculateNetworkRates(
		gnet.IOCountersStat{BytesRecv: 5000, BytesSent: 5000},
		gnet.IOCountersStat{BytesRecv: 100, BytesSent: 100},
		1,
	)
	if resetRx != 0 || resetTx != 0 {
		t.Errorf("counter reset should produce zero rates, got rx=%d tx=%d", resetRx, resetTx)
	}

	// 零采样间隔应直接返回 0
	zeroRx, zeroTx := calculateNetworkRates(previous, current, 0)
	if zeroRx != 0 || zeroTx != 0 {
		t.Errorf("zero duration should produce zero rates, got rx=%d tx=%d", zeroRx, zeroTx)
	}
}

// TestNetworkCollectorCollect 对真实系统执行一次采集，验证返回值的基本约束
func TestNetworkCollectorCollect(t *testing.T) {
	c := NewNetworkCollector()

	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("Collect() returned error: %v", err)
	}
	if len(metrics) == 0 {
		t.Skip("no non-loopback network interfaces available in this environment")
	}

	for _, metric := range metrics {
		if metric.Interface == "" {
			t.Error("interface name should not be empty")
		}
		if isLoopbackInterface(metric.Interface, nil) {
			t.Errorf("loopback interface should be excluded: %s", metric.Interface)
		}
		if metric.RxBytesTotal < 0 || metric.TxBytesTotal < 0 {
			t.Errorf("total traffic should not be negative: %+v", metric)
		}
		if metric.RxPackets < 0 || metric.TxPackets < 0 {
			t.Errorf("packet counts should not be negative: %+v", metric)
		}
		if metric.RxBytesPerSec < 0 || metric.TxBytesPerSec < 0 {
			t.Errorf("rates should not be negative: %+v", metric)
		}
	}

	// 第二次采集会基于上次快照计算速率，速率不应为负数
	if _, err := c.Collect(); err != nil {
		t.Fatalf("second Collect() returned error: %v", err)
	}
}
