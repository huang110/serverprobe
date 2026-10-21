package cache

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// KeyEventChannel 实时事件 Pub/Sub 频道
const KeyEventChannel = "agent:event"

// EventType 事件类型
const (
	EventTypeHeartbeat = "heartbeat"
	EventTypeAlert     = "alert"
)

// AgentEvent Agent 事件
type AgentEvent struct {
	Type      string `json:"type"`
	AgentID   string `json:"agent_id"`
	RuleID    uint64 `json:"rule_id,omitempty"`
	Severity  string `json:"severity,omitempty"`
	Message   string `json:"message,omitempty"`
	Status    string `json:"status,omitempty"`
	Timestamp int64  `json:"timestamp,omitempty"`
	FiredAt   int64  `json:"fired_at,omitempty"`
}

// PublishEvent 向 agent:event 频道发布事件
func (r *RedisClient) PublishEvent(ctx context.Context, event AgentEvent) error {
	if r == nil || r.client == nil {
		return nil
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	if err := r.client.Publish(ctx, KeyEventChannel, payload).Err(); err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	return nil
}

// SubscribeEvents 订阅 agent:event 频道
func (r *RedisClient) SubscribeEvents(ctx context.Context) *redis.PubSub {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Subscribe(ctx, KeyEventChannel)
}
