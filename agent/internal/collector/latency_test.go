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
	baseNow := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		target     config.LatencyTarget
		durations  []time.Duration
		failures   int
		wantTarget string
		wantLat    float64
		wantJitter float64
		wantLoss   float64
	}{
		{
			name:       "电信",
			target:     config.LatencyTarget{Name: "电信", Address: "10.0.0.1"},
			durations:  []time.Duration{30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
			wantTarget: "10.0.0.1:443",
			wantLat:    40,
			wantJitter: 20,
			wantLoss:   0,
		},
		{
			name:       "联通",
			target:     config.LatencyTarget{Name: "联通", Address: "10.0.0.2:443"},
			durations:  []time.Duration{10 * time.Millisecond, 20 * time.Millisecond},
			failures:   1,
			wantTarget: "10.0.0.2:443",
			wantLat:    15,
			wantJitter: 10,
			wantLoss:   100.0 / 3,
		},
		{
			name:       "移动",
			target:     config.LatencyTarget{Name: "移动", Address: "10.0.0.3:443"},
			failures:   3,
			wantTarget: "10.0.0.3:443",
			wantLat:    0,
			wantJitter: 0,
			wantLoss:   100,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := baseNow
			attempts := 0
			dialer := func(_, _ string, _ time.Duration) (net.Conn, error) {
				index := attempts
				attempts++
				if index < test.failures {
					return nil, errors.New("connection timeout")
				}
				sampleIndex := index - test.failures
				if sampleIndex >= 0 && sampleIndex < len(test.durations) {
					now = now.Add(test.durations[sampleIndex])
				}
				return latencyTestConn{}, nil
			}

			collector := NewLatencyCollector([]config.LatencyTarget{test.target},
				WithLatencyDialer(dialer),
				WithLatencyClock(func() time.Time { return now }),
				WithLatencyProbeInterval(0),
			)

			results := collector.Collect()
			if len(results) != 1 {
				t.Fatalf("result count = %d, want 1", len(results))
			}
			result := results[0]
			if result.Target != test.wantTarget {
				t.Errorf("target = %q, want %q", result.Target, test.wantTarget)
			}
			if math.Abs(result.LatencyMs-test.wantLat) > 1e-9 {
				t.Errorf("latency = %f, want %f", result.LatencyMs, test.wantLat)
			}
			if math.Abs(result.JitterMs-test.wantJitter) > 1e-9 {
				t.Errorf("jitter = %f, want %f", result.JitterMs, test.wantJitter)
			}
			if math.Abs(result.PacketLoss-test.wantLoss) > 0.01 {
				t.Errorf("packet loss = %f, want %f", result.PacketLoss, test.wantLoss)
			}
			if collector.LastCheckTime() != now.Unix() {
				t.Errorf("LastCheckTime = %d, want %d", collector.LastCheckTime(), now.Unix())
			}
		})
	}
}

// TestLatencyCollectorMultipleTargets 验证多目标并发采集不会丢失目标
func TestLatencyCollectorMultipleTargets(t *testing.T) {
	dialer := func(_, address string, _ time.Duration) (net.Conn, error) {
		if address == "10.0.0.3:443" {
			return nil, errors.New("connection timeout")
		}
		return latencyTestConn{}, nil
	}

	collector := NewLatencyCollector([]config.LatencyTarget{
		{Name: "电信", Address: "10.0.0.1"},
		{Name: "联通", Address: "10.0.0.2:443"},
		{Name: "移动", Address: "10.0.0.3:443"},
	},
		WithLatencyDialer(dialer),
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
	if byName["电信"].Target != "10.0.0.1:443" {
		t.Errorf("telecom target = %q, want 10.0.0.1:443", byName["电信"].Target)
	}
	if byName["移动"].PacketLoss != 100 {
		t.Errorf("mobile packet loss = %f, want 100", byName["移动"].PacketLoss)
	}
	if byName["联通"].PacketLoss != 0 {
		t.Errorf("unicom packet loss = %f, want 0", byName["联通"].PacketLoss)
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
	var dialMu sync.Mutex
	dialed := make(map[string]bool)
	dialer := func(_, address string, _ time.Duration) (net.Conn, error) {
		dialMu.Lock()
		dialed[address] = true
		dialMu.Unlock()
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

	if !dialed["8.8.8.8:53"] {
		t.Errorf("8.8.8.8 should use port 53, dialed: %v", dialed)
	}
	if !dialed["1.1.1.1:443"] {
		t.Errorf("1.1.1.1 should use port 443, dialed: %v", dialed)
	}
}
