package collector

import (
	"math"
	"testing"

	"github.com/shirou/gopsutil/v3/cpu"
)

func TestCalculateCPUPercentages(t *testing.T) {
	previous := cpu.TimesStat{
		User: 100, Nice: 10, System: 50, Idle: 800,
		Iowait: 20, Irq: 5, Softirq: 5, Steal: 10,
	}
	current := cpu.TimesStat{
		User: 150, Nice: 20, System: 70, Idle: 1600,
		Iowait: 40, Irq: 10, Softirq: 10, Steal: 20,
	}

	user, system, idle, iowait, steal := calculateCPUPercentages(current, &previous)

	tests := []struct {
		name string
		got  float64
		want float64
	}{
		{"user", user, 50.0 / 920 * 100},
		{"system", system, 20.0 / 920 * 100},
		{"idle", idle, 800.0 / 920 * 100},
		{"iowait", iowait, 20.0 / 920 * 100},
		{"steal", steal, 10.0 / 920 * 100},
	}
	for _, test := range tests {
		if math.Abs(test.got-test.want) > 1e-9 {
			t.Errorf("%s percent = %f, want %f", test.name, test.got, test.want)
		}
	}
}

func TestCalculateCPUPercentagesFirstSample(t *testing.T) {
	current := cpu.TimesStat{
		User: 100, System: 50, Idle: 850,
		Iowait: 10, Steal: 5, Nice: 0, Irq: 2, Softirq: 3,
	}

	user, system, idle, _, _ := calculateCPUPercentages(current, nil)
	total := 1020.0
	if math.Abs(user-100.0/total*100) > 1e-9 {
		t.Errorf("user percent = %f", user)
	}
	if math.Abs(system-50.0/total*100) > 1e-9 {
		t.Errorf("system percent = %f", system)
	}
	if math.Abs(idle-850.0/total*100) > 1e-9 {
		t.Errorf("idle percent = %f", idle)
	}
}

func TestCalculateCPUPercentagesCounterReset(t *testing.T) {
	previous := cpu.TimesStat{User: 100, System: 50, Idle: 850}
	current := cpu.TimesStat{User: 10, System: 20, Idle: 900}

	user, system, idle, _, _ := calculateCPUPercentages(current, &previous)
	if user != 0 {
		t.Errorf("user percent should be 0 after reset, got %f", user)
	}
	if system < 0 || idle < 0 {
		t.Errorf("percentages should not be negative: system=%f idle=%f", system, idle)
	}
}

func TestCPUCollectorCollect(t *testing.T) {
	collector := NewCPUCollector()

	first, err := collector.Collect()
	if err != nil {
		t.Fatalf("first Collect() failed: %v", err)
	}
	second, err := collector.Collect()
	if err != nil {
		t.Fatalf("second Collect() failed: %v", err)
	}

	for _, metrics := range []*CPUMetrics{first, second} {
		values := []float64{
			metrics.UsagePercent,
			metrics.UserPercent,
			metrics.SystemPercent,
			metrics.IdlePercent,
			metrics.IowaitPercent,
			metrics.StealPercent,
		}
		for _, value := range values {
			if value < 0 || value > 100 {
				t.Errorf("CPU percentage out of range: %+v", metrics)
				break
			}
			if math.IsNaN(value) {
				t.Errorf("CPU percentage is NaN: %+v", metrics)
				break
			}
		}
	}
}
