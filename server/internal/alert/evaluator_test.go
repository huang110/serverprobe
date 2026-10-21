package alert

import (
	"context"
	"testing"
	"time"

	"github.com/huang110/serverprobe/server/internal/model"
)

func TestCompareValue(t *testing.T) {
	tests := []struct {
		value     float64
		condition string
		threshold float64
		want      bool
	}{
		{90, "gt", 80, true},
		{80, "gt", 80, false},
		{80, "gte", 80, true},
		{70, "lt", 80, true},
		{80, "lte", 80, true},
		{80, "eq", 80, true},
		{81, "eq", 80, false},
		{80, "unknown", 80, false},
	}

	for _, test := range tests {
		if got := compareValue(test.value, test.condition, test.threshold); got != test.want {
			t.Errorf("compareValue(%.1f, %s, %.1f) = %v, want %v",
				test.value, test.condition, test.threshold, got, test.want)
		}
	}
}

func TestCurrentValue(t *testing.T) {
	redisClient := setupAlertTestRedis(t)
	evaluator := NewEvaluator(redisClient)

	if err := redisClient.SaveRealtimeMetrics("agent-1", map[string]interface{}{
		"cpu_usage":      55.5,
		"memory_percent": 70,
	}); err != nil {
		t.Fatalf("failed to save realtime metrics: %v", err)
	}
	if err := redisClient.SaveDiskMetrics("agent-1", map[string]interface{}{
		"/":     map[string]interface{}{"usage_percent": 60.0},
		"/data": map[string]interface{}{"usage_percent": 85.0},
	}); err != nil {
		t.Fatalf("failed to save disk metrics: %v", err)
	}

	value, err := evaluator.CurrentValue(&model.AlertRule{Metric: "cpu_usage"}, "agent-1")
	if err != nil || value != 55.5 {
		t.Errorf("cpu value = %v, err = %v", value, err)
	}

	value, err = evaluator.CurrentValue(&model.AlertRule{Metric: "disk_usage"}, "agent-1")
	if err != nil || value != 85 {
		t.Errorf("disk value = %v, err = %v", value, err)
	}

	if _, err := evaluator.CurrentValue(&model.AlertRule{Metric: "unknown"}, "agent-1"); err == nil {
		t.Error("unsupported metric should return an error")
	}
}

func TestShouldFireImmediate(t *testing.T) {
	redisClient := setupAlertTestRedis(t)
	evaluator := NewEvaluator(redisClient)
	ctx := context.Background()

	rule := &model.AlertRule{ID: 1, Duration: 0}
	result, err := evaluator.shouldFire(ctx, rule, "agent-1", true)
	if err != nil {
		t.Fatalf("shouldFire() failed: %v", err)
	}
	if result != ResultFiring {
		t.Errorf("result = %v, want firing", result)
	}

	// 已经 firing，不应重复触发
	result, err = evaluator.shouldFire(ctx, rule, "agent-1", true)
	if err != nil {
		t.Fatalf("shouldFire() failed: %v", err)
	}
	if result != ResultOK {
		t.Errorf("second result = %v, want ok", result)
	}
}

func TestShouldFireContinuous(t *testing.T) {
	redisClient := setupAlertTestRedis(t)
	evaluator := NewEvaluator(redisClient)
	ctx := context.Background()

	rule := &model.AlertRule{ID: 2, Duration: 5}

	result, err := evaluator.shouldFire(ctx, rule, "agent-1", true)
	if err != nil {
		t.Fatalf("shouldFire() failed: %v", err)
	}
	if result != ResultPending {
		t.Errorf("first result = %v, want pending", result)
	}

	// 模拟已经持续超过 duration
	if err := redisClient.SaveAlertState(rule.ID, "agent-1", map[string]interface{}{
		"first_triggered_at": time.Now().Add(-10 * time.Second).Unix(),
		"status":             "pending",
	}); err != nil {
		t.Fatalf("failed to save alert state: %v", err)
	}

	result, err = evaluator.shouldFire(ctx, rule, "agent-1", true)
	if err != nil {
		t.Fatalf("shouldFire() failed: %v", err)
	}
	if result != ResultFiring {
		t.Errorf("result after duration = %v, want firing", result)
	}

	// 条件不再满足，应恢复
	result, err = evaluator.shouldFire(ctx, rule, "agent-1", false)
	if err != nil {
		t.Fatalf("shouldFire() failed: %v", err)
	}
	if result != ResultResolved {
		t.Errorf("result = %v, want resolved", result)
	}

	// 恢复后状态已删除
	result, err = evaluator.shouldFire(ctx, rule, "agent-1", false)
	if err != nil {
		t.Fatalf("shouldFire() failed: %v", err)
	}
	if result != ResultOK {
		t.Errorf("result = %v, want ok", result)
	}
}

func TestExtractDiskUsage(t *testing.T) {
	value, err := extractDiskUsage(`{"usage_percent":65.5}`)
	if err != nil || value != 65.5 {
		t.Errorf("value = %v, err = %v", value, err)
	}

	if _, err := extractDiskUsage(`not-json`); err == nil {
		t.Error("invalid json should return an error")
	}
}
