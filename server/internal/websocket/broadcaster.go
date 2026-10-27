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
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/model"
)

const (
	// broadcastInterval 周期性推送在线指标间隔
	broadcastInterval = 10 * time.Second

	// offlineCheckInterval 离线巡检间隔
	offlineCheckInterval = 15 * time.Second

	// offlineThreshold Agent 超过该秒数无心跳视为离线
	offlineThreshold = 90

	// onlineAgentThreshold 在线 Agent 判定阈值（秒）
	onlineAgentThreshold = 300
)

// Broadcaster 订阅 Redis 事件并广播到 WebSocket 客户端
type Broadcaster struct {
	hub    *Hub
	redis  *cache.RedisClient
	db     *gorm.DB
	stopCh chan struct{}
	doneCh chan struct{}

	stopOnce sync.Once
}

// NewBroadcaster 创建广播器
func NewBroadcaster(hub *Hub, redisClient *cache.RedisClient, db *gorm.DB) *Broadcaster {
	return &Broadcaster{
		hub:    hub,
		redis:  redisClient,
		db:     db,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// Start 订阅事件频道并运行离线巡检，阻塞运行直到 Stop
func (b *Broadcaster) Start() {
	defer close(b.doneCh)

	ctx := context.Background()

	var channel <-chan *redis.Message
	if b.redis != nil {
		pubsub := b.redis.SubscribeEvents(ctx)
		if pubsub != nil {
			defer func() {
				if err := pubsub.Close(); err != nil {
					log.Printf("warning: failed to close event subscription: %v", err)
				}
			}()
			channel = pubsub.Channel()
		} else {
			log.Printf("warning: failed to subscribe to agent events")
		}
	} else {
		log.Printf("warning: redis client is nil, websocket event broadcast disabled")
	}

	ticker := time.NewTicker(broadcastInterval)
	defer ticker.Stop()

	offlineTicker := time.NewTicker(offlineCheckInterval)
	defer offlineTicker.Stop()

	// 启动时先清理一次历史离线 Agent
	b.checkOfflineAgents(ctx)

	for {
		select {
		case message, ok := <-channel:
			if !ok {
				return
			}
			b.handleMessage(ctx, []byte(message.Payload))
		case <-ticker.C:
			b.broadcastOnlineAgents(ctx)
		case <-offlineTicker.C:
			b.checkOfflineAgents(ctx)
		case <-b.stopCh:
			return
		}
	}
}

// checkOfflineAgents 扫描超时未心跳的 Agent，更新状态并广播
func (b *Broadcaster) checkOfflineAgents(ctx context.Context) {
	if b.db == nil {
		return
	}

	threshold := time.Now().Unix() - offlineThreshold
	var agents []model.Agent
	if err := b.db.WithContext(ctx).
		Where("status = ? AND last_heartbeat_at < ?", "online", threshold).
		Find(&agents).Error; err != nil {
		log.Printf("warning: failed to query offline agents: %v", err)
		return
	}

	for _, agent := range agents {
		result := b.db.WithContext(ctx).Model(&model.Agent{}).
			Where("id = ? AND status = ?", agent.ID, "online").
			Update("status", "offline")
		if result.Error != nil {
			log.Printf("warning: failed to mark agent %s offline: %v", agent.ID, result.Error)
			continue
		}
		if result.RowsAffected == 0 {
			continue
		}

		if b.redis != nil {
			if err := b.redis.MarkAgentOffline(agent.ID); err != nil {
				log.Printf("warning: failed to update redis status for offline agent %s: %v", agent.ID, err)
			}
		}

		b.hub.Broadcast(buildAgentStatusMessage(agent.ID, "offline", agent.LastHeartbeatAt))
		log.Printf("agent marked offline: %s", agent.ID)
	}
}

// broadcastOnlineAgents 周期性推送所有在线 Agent 的实时指标
func (b *Broadcaster) broadcastOnlineAgents(ctx context.Context) {
	if b.redis == nil {
		return
	}
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
		latency, err := b.redis.GetLatencyItems(agentID)
		if err != nil {
			latency = nil
		}
		b.hub.Broadcast(buildRealtimeMessage(agentID, metrics, now, latency))
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
		latency, latencyErr := b.redis.GetLatencyItems(event.AgentID)
		if latencyErr != nil {
			log.Printf("warning: failed to load latency metrics for %s: %v", event.AgentID, latencyErr)
		}
		b.hub.Broadcast(buildRealtimeMessage(event.AgentID, metrics, event.Timestamp, latency))
		b.hub.Broadcast(buildAgentStatusMessage(event.AgentID, "online", event.Timestamp))
	case cache.EventTypeAlert:
		b.hub.Broadcast(buildAlertMessage(event))
	default:
		log.Printf("warning: unknown agent event type %q", event.Type)
	}
}

// buildRealtimeMessage 构造实时指标消息。
// latencyItems 为可选参数，不传时输出空数组。
func buildRealtimeMessage(agentID string, metrics map[string]string, timestamp int64, latencyItems ...[]model.LatencyItem) []byte {
	var latency []model.LatencyItem
	if len(latencyItems) > 0 {
		latency = latencyItems[0]
	}
	if latency == nil {
		latency = make([]model.LatencyItem, 0)
	}

	data := map[string]interface{}{
		"agent_id":       agentID,
		"cpu_usage":      parseFloat(metrics["cpu_usage"]),
		"cpu_steal":      parseFloat(metrics["cpu_steal"]),
		"memory_percent": parseFloat(metrics["memory_percent"]),
		"swap_percent":   parseFloat(metrics["swap_percent"]),
		"load1":          parseFloat(metrics["load1"]),
		"net_rx_bytes":   parseInt64(metrics["net_rx_bytes"]),
		"net_tx_bytes":   parseInt64(metrics["net_tx_bytes"]),
		"net_rx_total":   parseInt64(metrics["net_rx_total"]),
		"net_tx_total":   parseInt64(metrics["net_tx_total"]),
		"uptime":         parseInt64(metrics["uptime"]),
		"latency":        latency,
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
