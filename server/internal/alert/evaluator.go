package alert

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/model"
)

// EvaluationResult 规则评估结果
type EvaluationResult string

const (
	// ResultOK 未触发或已处理
	ResultOK EvaluationResult = "ok"

	// ResultPending 已满足条件但未达到连续触发时长
	ResultPending EvaluationResult = "pending"

	// ResultFiring 触发告警
	ResultFiring EvaluationResult = "firing"

	// ResultResolved 告警恢复
	ResultResolved EvaluationResult = "resolved"
)

// Evaluator 告警规则评估器
type Evaluator struct {
	redis *cache.RedisClient
}

// NewEvaluator 创建评估器
func NewEvaluator(redisClient *cache.RedisClient) *Evaluator {
	return &Evaluator{redis: redisClient}
}

// CurrentValue 读取规则对应指标的当前值
func (e *Evaluator) CurrentValue(rule *model.AlertRule, agentID string) (float64, error) {
	if e.redis == nil {
		return 0, errors.New("redis is required")
	}
	if rule == nil {
		return 0, errors.New("rule is required")
	}

	switch rule.Metric {
	case "cpu_usage", "memory_percent", "load1":
		metrics, err := e.redis.GetRealtimeMetrics(agentID)
		if err != nil {
			return 0, err
		}
		value, ok := metrics[rule.Metric]
		if !ok {
			return 0, fmt.Errorf("metric %s not found for agent %s", rule.Metric, agentID)
		}
		return strconv.ParseFloat(value, 64)

	case "disk_usage":
		disks, err := e.redis.GetDiskMetrics(agentID)
		if err != nil {
			return 0, err
		}
		if len(disks) == 0 {
			return 0, fmt.Errorf("no disk metrics for agent %s", agentID)
		}

		var maxUsage float64
		hasValue := false
		for _, payload := range disks {
			value, err := extractDiskUsage(payload)
			if err != nil {
				continue
			}
			if !hasValue || value > maxUsage {
				maxUsage = value
				hasValue = true
			}
		}
		if !hasValue {
			return 0, fmt.Errorf("no valid disk usage for agent %s", agentID)
		}
		return maxUsage, nil

	default:
		return 0, fmt.Errorf("unsupported metric %s", rule.Metric)
	}
}

// Evaluate 评估规则并返回状态和当前值
func (e *Evaluator) Evaluate(ctx context.Context, rule *model.AlertRule, agentID string) (EvaluationResult, float64, error) {
	value, err := e.CurrentValue(rule, agentID)
	if err != nil {
		return ResultOK, 0, err
	}

	triggered := compareValue(value, rule.Condition, rule.Threshold)
	result, err := e.shouldFire(ctx, rule, agentID, triggered)
	if err != nil {
		return ResultOK, value, err
	}
	return result, value, nil
}

// shouldFire 根据连续触发时长判断是否应该触发或恢复
func (e *Evaluator) shouldFire(ctx context.Context, rule *model.AlertRule, agentID string, triggered bool) (EvaluationResult, error) {
	if e.redis == nil {
		return ResultOK, errors.New("redis is required")
	}

	state, err := e.redis.GetAlertState(rule.ID, agentID)
	if err != nil {
		return ResultOK, err
	}

	now := time.Now().Unix()
	status := state["status"]

	if !triggered {
		if status == "firing" {
			if err := e.redis.DeleteAlertState(rule.ID, agentID); err != nil {
				return ResultOK, err
			}
			return ResultResolved, nil
		}
		if len(state) > 0 {
			if err := e.redis.DeleteAlertState(rule.ID, agentID); err != nil {
				return ResultOK, err
			}
		}
		return ResultOK, nil
	}

	// duration <= 0 时立即触发
	if rule.Duration <= 0 {
		if status == "firing" {
			return ResultOK, nil
		}
		if err := e.redis.SaveAlertState(rule.ID, agentID, map[string]interface{}{
			"status":          "firing",
			"last_checked_at": now,
		}); err != nil {
			return ResultOK, err
		}
		return ResultFiring, nil
	}

	firstTriggeredAt, err := strconv.ParseInt(state["first_triggered_at"], 10, 64)
	if err != nil {
		// 首次满足条件，记录起始时间
		if err := e.redis.SaveAlertState(rule.ID, agentID, map[string]interface{}{
			"first_triggered_at": now,
			"last_checked_at":    now,
			"status":             "pending",
		}); err != nil {
			return ResultOK, err
		}
		return ResultPending, nil
	}

	if now-firstTriggeredAt < int64(rule.Duration) {
		if err := e.redis.SaveAlertState(rule.ID, agentID, map[string]interface{}{
			"last_checked_at": now,
		}); err != nil {
			return ResultOK, err
		}
		return ResultPending, nil
	}

	if status == "firing" {
		return ResultOK, nil
	}

	if err := e.redis.SaveAlertState(rule.ID, agentID, map[string]interface{}{
		"status":          "firing",
		"last_checked_at": now,
	}); err != nil {
		return ResultOK, err
	}
	return ResultFiring, nil
}

// compareValue 比较当前值和阈值
func compareValue(value float64, condition string, threshold float64) bool {
	switch condition {
	case "gt":
		return value > threshold
	case "gte":
		return value >= threshold
	case "lt":
		return value < threshold
	case "lte":
		return value <= threshold
	case "eq":
		return value == threshold
	default:
		return false
	}
}

// extractDiskUsage 从磁盘缓存 JSON 中提取使用率
func extractDiskUsage(payload string) (float64, error) {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		return 0, err
	}

	switch value := data["usage_percent"].(type) {
	case float64:
		return value, nil
	case string:
		return strconv.ParseFloat(value, 64)
	default:
		return 0, errors.New("usage_percent not found")
	}
}
