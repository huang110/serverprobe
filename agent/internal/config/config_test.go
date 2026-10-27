package config

import (
	"testing"
	"time"
)

func TestDefaultIPQualityConfig(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.IPQuality.Enabled {
		t.Error("IP quality should be enabled by default")
	}
	if cfg.IPQuality.CheckInterval != 15 {
		t.Errorf("check interval = %d, want 15", cfg.IPQuality.CheckInterval)
	}
	if got := cfg.GetIPQualityInterval(); got != 15*time.Second {
		t.Errorf("GetIPQualityInterval() = %s, want 15s", got)
	}
	if len(cfg.IPQuality.LatencyTargets) != 5 {
		t.Fatalf("latency target count = %d, want 5", len(cfg.IPQuality.LatencyTargets))
	}

	expected := map[string]string{
		"电信":         "202.102.192.68:53",
		"联通":         "210.22.70.3:53",
		"移动":         "211.136.192.6:53",
		"阿里DNS":      "223.5.5.5:443",
		"Cloudflare": "1.1.1.1:443",
	}
	for _, target := range cfg.IPQuality.LatencyTargets {
		want, ok := expected[target.Name]
		if !ok {
			t.Errorf("unexpected latency target: %+v", target)
			continue
		}
		if target.Address != want {
			t.Errorf("target %s address = %q, want %q", target.Name, target.Address, want)
		}
	}
}

func TestGetIPQualityIntervalDefault(t *testing.T) {
	cfg := &Config{}
	if got := cfg.GetIPQualityInterval(); got != 15*time.Second {
		t.Errorf("GetIPQualityInterval() = %s, want 15s", got)
	}

	var nilConfig *Config
	if got := nilConfig.GetIPQualityInterval(); got != 15*time.Second {
		t.Errorf("nil config GetIPQualityInterval() = %s, want 15s", got)
	}
}
