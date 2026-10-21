package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	// realtimeMetricsTTL 实时指标缓存时间
	realtimeMetricsTTL = 5 * time.Minute

	// onlineAgentsTTL 在线 Agent 集合的兜底过期时间
	onlineAgentsTTL = 10 * time.Minute

	// agentDetailTTL Agent 详情缓存时间
	agentDetailTTL = 10 * time.Minute

	// alertStateTTL 告警状态缓存时间
	alertStateTTL = 24 * time.Hour
)

// SaveRealtimeMetrics 保存实时指标到 Redis Hash。
// Hash 字段示例：cpu_usage、memory_used、memory_percent、load1、status、last_heartbeat_at。
func (r *RedisClient) SaveRealtimeMetrics(agentID string, metrics map[string]interface{}) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}
	if len(metrics) == 0 {
		return nil
	}

	key := realtimeMetricsKey(agentID)
	fields := make(map[string]interface{}, len(metrics))
	for field, value := range metrics {
		fields[field] = fmt.Sprint(value)
	}

	pipe := r.client.Pipeline()
	pipe.HSet(r.ctx, key, fields)
	pipe.Expire(r.ctx, key, realtimeMetricsTTL)
	_, err := pipe.Exec(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to save realtime metrics: %w", err)
	}
	return nil
}

// GetRealtimeMetrics 获取 Agent 的实时指标
func (r *RedisClient) GetRealtimeMetrics(agentID string) (map[string]string, error) {
	if agentID == "" {
		return nil, errors.New("agent id is required")
	}
	return r.client.HGetAll(r.ctx, realtimeMetricsKey(agentID)).Result()
}

// UpdateOnlineAgent 更新 Agent 在线状态，score 为最后心跳时间戳
func (r *RedisClient) UpdateOnlineAgent(agentID string, lastHeartbeat int64) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}

	pipe := r.client.Pipeline()
	pipe.ZAdd(r.ctx, KeyOnlineAgents, redis.Z{
		Score:  float64(lastHeartbeat),
		Member: agentID,
	})
	pipe.Expire(r.ctx, KeyOnlineAgents, onlineAgentsTTL)
	_, err := pipe.Exec(r.ctx)
	if err != nil {
		return fmt.Errorf("failed to update online agent: %w", err)
	}
	return nil
}

// GetOnlineAgents 获取最近 threshold 秒内有过心跳的 Agent
func (r *RedisClient) GetOnlineAgents(threshold int64) ([]string, error) {
	if threshold <= 0 {
		return nil, errors.New("threshold must be positive")
	}

	minScore := time.Now().Unix() - threshold
	return r.client.ZRangeByScore(r.ctx, KeyOnlineAgents, &redis.ZRangeBy{
		Min: strconv.FormatInt(minScore, 10),
		Max: "+inf",
	}).Result()
}

// RemoveOfflineAgents 清理心跳超过 threshold 秒的 Agent，返回删除数量
func (r *RedisClient) RemoveOfflineAgents(threshold int64) (int64, error) {
	if threshold <= 0 {
		return 0, errors.New("threshold must be positive")
	}

	maxScore := time.Now().Unix() - threshold
	return r.client.ZRemRangeByScore(
		r.ctx,
		KeyOnlineAgents,
		"-inf",
		strconv.FormatInt(maxScore, 10),
	).Result()
}

// SaveDiskMetrics 保存磁盘指标，字段为 mount_point -> JSON
func (r *RedisClient) SaveDiskMetrics(agentID string, disks map[string]interface{}) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}

	fields := make(map[string]interface{}, len(disks))
	for mountPoint, data := range disks {
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal disk metric for %s: %w", mountPoint, err)
		}
		fields[mountPoint] = string(encoded)
	}

	key := realtimeDiskKey(agentID)
	pipe := r.client.Pipeline()
	pipe.Del(r.ctx, key)
	if len(fields) > 0 {
		pipe.HSet(r.ctx, key, fields)
		pipe.Expire(r.ctx, key, realtimeMetricsTTL)
	}
	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("failed to save disk metrics: %w", err)
	}
	return nil
}

// GetDiskMetrics 获取磁盘指标，返回 mount_point -> JSON 字符串
func (r *RedisClient) GetDiskMetrics(agentID string) (map[string]string, error) {
	if agentID == "" {
		return nil, errors.New("agent id is required")
	}
	return r.client.HGetAll(r.ctx, realtimeDiskKey(agentID)).Result()
}

// SaveNetworkMetrics 保存网络指标，字段为 interface -> JSON
func (r *RedisClient) SaveNetworkMetrics(agentID string, networks map[string]interface{}) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}

	fields := make(map[string]interface{}, len(networks))
	for interfaceName, data := range networks {
		encoded, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal network metric for %s: %w", interfaceName, err)
		}
		fields[interfaceName] = string(encoded)
	}

	key := realtimeNetworkKey(agentID)
	pipe := r.client.Pipeline()
	pipe.Del(r.ctx, key)
	if len(fields) > 0 {
		pipe.HSet(r.ctx, key, fields)
		pipe.Expire(r.ctx, key, realtimeMetricsTTL)
	}
	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("failed to save network metrics: %w", err)
	}
	return nil
}

// GetNetworkMetrics 获取网络指标，返回 interface -> JSON 字符串
func (r *RedisClient) GetNetworkMetrics(agentID string) (map[string]string, error) {
	if agentID == "" {
		return nil, errors.New("agent id is required")
	}
	return r.client.HGetAll(r.ctx, realtimeNetworkKey(agentID)).Result()
}

// SaveAgentDetail 缓存 Agent 详情（JSON）
func (r *RedisClient) SaveAgentDetail(agentID string, detail interface{}) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}

	encoded, err := json.Marshal(detail)
	if err != nil {
		return fmt.Errorf("failed to marshal agent detail: %w", err)
	}

	if err := r.client.Set(r.ctx, agentDetailKey(agentID), encoded, agentDetailTTL).Err(); err != nil {
		return fmt.Errorf("failed to save agent detail: %w", err)
	}
	return nil
}

// GetAgentDetail 获取 Agent 详情，found=false 表示缓存不存在
func (r *RedisClient) GetAgentDetail(agentID string, dest interface{}) (bool, error) {
	if agentID == "" {
		return false, errors.New("agent id is required")
	}

	data, err := r.client.Get(r.ctx, agentDetailKey(agentID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return false, nil
		}
		return false, fmt.Errorf("failed to get agent detail: %w", err)
	}

	if err := json.Unmarshal(data, dest); err != nil {
		return false, fmt.Errorf("failed to unmarshal agent detail: %w", err)
	}
	return true, nil
}

// SaveAlertState 保存告警规则状态（连续触发计数、首次触发时间等）
func (r *RedisClient) SaveAlertState(ruleID uint64, agentID string, state map[string]interface{}) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}
	if len(state) == 0 {
		return nil
	}

	fields := make(map[string]interface{}, len(state))
	for field, value := range state {
		fields[field] = fmt.Sprint(value)
	}

	key := alertStateKey(ruleID, agentID)
	pipe := r.client.Pipeline()
	pipe.HSet(r.ctx, key, fields)
	pipe.Expire(r.ctx, key, alertStateTTL)
	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("failed to save alert state: %w", err)
	}
	return nil
}

// GetAlertState 获取告警规则状态
func (r *RedisClient) GetAlertState(ruleID uint64, agentID string) (map[string]string, error) {
	if agentID == "" {
		return nil, errors.New("agent id is required")
	}
	return r.client.HGetAll(r.ctx, alertStateKey(ruleID, agentID)).Result()
}

// DeleteAlertState 删除告警规则状态（告警恢复时调用）
func (r *RedisClient) DeleteAlertState(ruleID uint64, agentID string) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}
	return r.client.Del(r.ctx, alertStateKey(ruleID, agentID)).Err()
}

// DeleteAgentCache 删除 Agent 的全部实时缓存，并从在线集合移除
func (r *RedisClient) DeleteAgentCache(agentID string) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}

	pipe := r.client.Pipeline()
	pipe.Del(
		r.ctx,
		realtimeMetricsKey(agentID),
		realtimeDiskKey(agentID),
		realtimeNetworkKey(agentID),
		agentDetailKey(agentID),
	)
	pipe.ZRem(r.ctx, KeyOnlineAgents, agentID)
	if _, err := pipe.Exec(r.ctx); err != nil {
		return fmt.Errorf("failed to delete agent cache: %w", err)
	}
	return nil
}

func realtimeMetricsKey(agentID string) string {
	return strings.ReplaceAll(KeyRealtimeMetrics, "{agent_id}", agentID)
}

func realtimeDiskKey(agentID string) string {
	return strings.ReplaceAll(KeyRealtimeDisk, "{agent_id}", agentID)
}

func realtimeNetworkKey(agentID string) string {
	return strings.ReplaceAll(KeyRealtimeNetwork, "{agent_id}", agentID)
}

func agentDetailKey(agentID string) string {
	return strings.ReplaceAll(KeyAgentDetail, "{agent_id}", agentID)
}

func alertStateKey(ruleID uint64, agentID string) string {
	key := strings.ReplaceAll(KeyAlertState, "{rule_id}", strconv.FormatUint(ruleID, 10))
	return strings.ReplaceAll(key, "{agent_id}", agentID)
}
