package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/huang110/serverprobe/server/internal/cache"
)

const (
	// broadcastInterval 周期性推送在线指标间隔
	broadcastInterval = 30 * time.Second

	// onlineAgentThreshold 在线 Agent 判定阈值（秒）
	onlineAgentThreshold = 300
)

// Broadcaster 订阅 Redis 事件并广播到 WebSocket 客户端
type Broadcaster struct {
	hub    *Hub
	redis  *cache.RedisClient
	stopCh chan struct{}
	doneCh chan struct{}

	stopOnce sync.Once
}

// NewBroadcaster 创建广播器
func NewBroadcaster(hub *Hub, redisClient *cache.RedisClient) *Broadcaster {
	return &Broadcaster{
		hub:    hub,
		redis:  redisClient,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start 订阅事件频道，阻塞运行直到 Stop
func (b *Broadcaster) Start() {
	defer close(b.doneCh)

	if b.redis == nil {
		log.Printf("warning: redis client is nil, websocket broadcaster disabled")
		return
	}

	ctx := context.Background()
	pubsub := b.redis.SubscribeEvents(ctx)
	if pubsub == nil {
		log.Printf("warning: failed to subscribe to agent events")
		return
	}
	defer func() {
		if err := pubsub.Close(); err != nil {
			log.Printf("warning: failed to close event subscription: %v", err)
		}
	}()

	channel := pubsub.Channel()
	ticker := time.NewTicker(broadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case message, ok := <-channel:
			if !ok {
				return
			}
			b.handleMessage(ctx, []byte(message.Payload))
		case <-ticker.C:
			b.broadcastOnlineAgents(ctx)
		case <-b.stopCh:
			return
		}
	}
}

// broadcastOnlineAgents 周期性推送所有在线 Agent 的实时指标
func (b *Broadcaster) broadcastOnlineAgents(ctx context.Context) {
	agents, err := b.redis.GetOnlineAgents(onlineAgentThreshold)
	if err != nil {
		log.Printf("warning: failed to load online agents: %v", err)
		return
	}

	now := time.Now().Unix()
	for _, agentID := range agents {
		metrics, err := b.redis.GetRealtimeMetrics(agentID)
		if err != nil {
			continue
		}
		b.hub.Broadcast(buildRealtimeMessage(agentID, metrics, now))
		b.hub.Broadcast(buildAgentStatusMessage(agentID, "online", now))
	}
}

// Stop 停止广播器
func (b *Broadcaster) Stop() {
	b.stopOnce.Do(func() {
		close(b.stopCh)
	})
	<-b.doneCh
}

// handleMessage 解析事件并广播
func (b *Broadcaster) handleMessage(ctx context.Context, payload []byte) {
	var event cache.AgentEvent
	if err := json.Unmarshal(payload, &event); err != nil {
		log.Printf("warning: failed to unmarshal agent event: %v", err)
		return
	}

	switch event.Type {
	case cache.EventTypeHeartbeat:
		metrics, err := b.redis.GetRealtimeMetrics(event.AgentID)
		if err != nil && !errors.Is(err, redis.Nil) {
			log.Printf("warning: failed to load realtime metrics for %s: %v", event.AgentID, err)
		}
		b.hub.Broadcast(buildRealtimeMessage(event.AgentID, metrics, event.Timestamp))
		b.hub.Broadcast(buildAgentStatusMessage(event.AgentID, "online", event.Timestamp))
	case cache.EventTypeAlert:
		b.hub.Broadcast(buildAlertMessage(event))
	default:
		log.Printf("warning: unknown agent event type %q", event.Type)
	}
}

// buildRealtimeMessage 构造实时指标消息
func buildRealtimeMessage(agentID string, metrics map[string]string, timestamp int64) []byte {
	data := map[string]interface{}{
		"agent_id":       agentID,
		"cpu_usage":      parseFloat(metrics["cpu_usage"]),
		"memory_percent": parseFloat(metrics["memory_percent"]),
		"load1":          parseFloat(metrics["load1"]),
		"net_rx_bytes":   parseInt64(metrics["net_rx_bytes"]),
		"net_tx_bytes":   parseInt64(metrics["net_tx_bytes"]),
		"timestamp":      timestamp,
	}
	if data["timestamp"].(int64) == 0 {
		data["timestamp"] = time.Now().Unix()
	}

	payload, _ := json.Marshal(outboundMessage{Type: "realtime_metrics", Data: data})
	return payload
}

// buildAgentStatusMessage 构造 Agent 状态消息
func buildAgentStatusMessage(agentID, status string, lastHeartbeatAt int64) []byte {
	payload, _ := json.Marshal(outboundMessage{
		Type: "agent_status",
		Data: map[string]interface{}{
			"agent_id":          agentID,
			"status":            status,
			"last_heartbeat_at": lastHeartbeatAt,
		},
	})
	return payload
}

// buildAlertMessage 构造告警消息
func buildAlertMessage(event cache.AgentEvent) []byte {
	payload, _ := json.Marshal(outboundMessage{
		Type: "alert",
		Data: map[string]interface{}{
			"rule_id":  event.RuleID,
			"agent_id": event.AgentID,
			"severity": event.Severity,
			"message":  event.Message,
			"fired_at": event.FiredAt,
		},
	})
	return payload
}

func parseFloat(value string) float64 {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func parseInt64(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}
