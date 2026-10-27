package collector

import (
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/huang110/serverprobe/agent/internal/config"
)

// latencyTestConn 最小化的 net.Conn 实现
type latencyTestConn struct{}

func (latencyTestConn) Read([]byte) (int, error)         { return 0, io.EOF }
func (latencyTestConn) Write(p []byte) (int, error)      { return len(p), nil }
func (latencyTestConn) Close() error                     { return nil }
func (latencyTestConn) LocalAddr() net.Addr              { return nil }
func (latencyTestConn) RemoteAddr() net.Addr             { return nil }
func (latencyTestConn) SetDeadline(time.Time) error      { return nil }
func (latencyTestConn) SetReadDeadline(time.Time) error  { return nil }
func (latencyTestConn) SetWriteDeadline(time.Time) error { return nil }

func TestNormalizeLatencyAddress(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"223.5.5.5:443", "223.5.5.5:443"},
		{"223.5.5.5", "223.5.5.5:443"},
		{"1.1.1.1", "1.1.1.1:443"},
		{"8.8.8.8", "8.8.8.8:53"},
		{"114.114.114.114", "114.114.114.114:53"},
		{" 202.102.192.68:53 ", "202.102.192.68:53"},
		{"2001:4860:4860::8888", "[2001:4860:4860::8888]:443"},
		{"[2001:4860:4860::8888]:443", "[2001:4860:4860::8888]:443"},
		{"", ""},
	}

	for _, test := range tests {
		if got := normalizeLatencyAddress(test.input); got != test.want {
			t.Errorf("normalizeLatencyAddress(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestLatencyCollectorCollect(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	var nowMu sync.Mutex

	durations := map[string][]time.Duration{
		"10.0.0.1:443": {30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
		"10.0.0.2:443": {10 * time.Millisecond, 20 * time.Millisecond},
	}
	failures := map[string]int{"10.0.0.2:443": 1, "10.0.0.3:443": 3}
	callCounts := make(map[string]int)

	dialer := func(_, address string, _ time.Duration) (net.Conn, error) {
		nowMu.Lock()
		defer nowMu.Unlock()

		index := callCounts[address]
		callCounts[address] = index + 1

		if index < failures[address] {
			return nil, errors.New("connection timeout")
		}

		samples := durations[address]
		sampleIndex := index - failures[address]
		if sampleIndex >= 0 && sampleIndex < len(samples) {
			now = now.Add(samples[sampleIndex])
		}
		return latencyTestConn{}, nil
	}

	targets := []config.LatencyTarget{
		{Name: "电信", Address: "10.0.0.1"},
		{Name: "联通", Address: "10.0.0.2:443"},
		{Name: "移动", Address: "10.0.0.3:443"},
	}
	collector := NewLatencyCollector(targets,
		WithLatencyDialer(dialer),
		WithLatencyClock(func() time.Time {
			nowMu.Lock()
			defer nowMu.Unlock()
			return now
		}),
		WithLatencyProbeInterval(0),
	)

	results := collector.Collect()
	if len(results) != 3 {
		t.Fatalf("result count = %d, want 3", len(results))
	}

	byName := make(map[string]LatencyResult, len(results))
	for _, result := range results {
		byName[result.TargetName] = result
	}

	telecom := byName["电信"]
	if telecom.Target != "10.0.0.1:443" {
		t.Errorf("telecom target = %q, want 10.0.0.1:443", telecom.Target)
	}
	if math.Abs(telecom.LatencyMs-40) > 1e-9 {
		t.Errorf("telecom latency = %f, want 40", telecom.LatencyMs)
	}
	if math.Abs(telecom.JitterMs-20) > 1e-9 {
		t.Errorf("telecom jitter = %f, want 20", telecom.JitterMs)
	}
	if telecom.PacketLoss != 0 {
		t.Errorf("telecom packet loss = %f, want 0", telecom.PacketLoss)
	}

	unicom := byName["联通"]
	if math.Abs(unicom.LatencyMs-15) > 1e-9 {
		t.Errorf("unicom latency = %f, want 15", unicom.LatencyMs)
	}
	if math.Abs(unicom.JitterMs-10) > 1e-9 {
		t.Errorf("unicom jitter = %f, want 10", unicom.JitterMs)
	}
	if math.Abs(unicom.PacketLoss-100.0/3) > 0.01 {
		t.Errorf("unicom packet loss = %f, want 33.33", unicom.PacketLoss)
	}

	mobile := byName["移动"]
	if mobile.LatencyMs != 0 || mobile.JitterMs != 0 {
		t.Errorf("mobile latency/jitter = %f/%f, want 0/0", mobile.LatencyMs, mobile.JitterMs)
	}
	if mobile.PacketLoss != 100 {
		t.Errorf("mobile packet loss = %f, want 100", mobile.PacketLoss)
	}

	if collector.LastCheckTime() != now.Unix() {
		t.Errorf("LastCheckTime = %d, want %d", collector.LastCheckTime(), now.Unix())
	}
}

func TestLatencyCollectorCache(t *testing.T) {
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	var dialCount int

	dialer := func(_, _ string, _ time.Duration) (net.Conn, error) {
		dialCount++
		return latencyTestConn{}, nil
	}

	collector := NewLatencyCollector(
		[]config.LatencyTarget{{Name: "cache", Address: "10.0.0.9:443"}},
		WithLatencyDialer(dialer),
		WithLatencyClock(func() time.Time { return now }),
		WithLatencyProbeInterval(0),
	)

	first := collector.Collect()
	if len(first) != 1 || first[0].PacketLoss != 0 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	if dialCount != defaultLatencyProbeCount {
		t.Fatalf("dial count = %d, want %d", dialCount, defaultLatencyProbeCount)
	}

	// 未超过 checkInterval 时直接返回缓存，不应再次拨号
	collector.Collect()
	if dialCount != defaultLatencyProbeCount {
		t.Errorf("dial count after cached collect = %d, want %d", dialCount, defaultLatencyProbeCount)
	}

	// 超过 checkInterval 后重新探测
	now = now.Add(defaultLatencyCheckInterval + time.Second)
	collector.Collect()
	if dialCount != defaultLatencyProbeCount*2 {
		t.Errorf("dial count after refresh = %d, want %d", dialCount, defaultLatencyProbeCount*2)
	}
}

func TestLatencyCollectorEmptyTargets(t *testing.T) {
	collector := NewLatencyCollector(nil)
	if results := collector.Collect(); results != nil {
		t.Errorf("results = %+v, want nil", results)
	}
}

func TestLatencyCollectorNoPortDefault(t *testing.T) {
	var dialed []string
	dialer := func(_, address string, _ time.Duration) (net.Conn, error) {
		dialed = append(dialed, address)
		return latencyTestConn{}, nil
	}

	collector := NewLatencyCollector(
		[]config.LatencyTarget{
			{Name: "dns", Address: "8.8.8.8"},
			{Name: "https", Address: "1.1.1.1"},
		},
		WithLatencyDialer(dialer),
		WithLatencyProbeInterval(0),
	)
	collector.Collect()

	seen := make(map[string]bool, len(dialed))
	for _, address := range dialed {
		seen[address] = true
	}
	if !seen["8.8.8.8:53"] {
		t.Errorf("8.8.8.8 should use port 53, dialed: %v", dialed)
	}
	if !seen["1.1.1.1:443"] {
		t.Errorf("1.1.1.1 should use port 443, dialed: %v", dialed)
	}
}
