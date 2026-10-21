package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key 模板
const (
	// KeyRealtimeMetrics 实时指标 Hash，字段见 RealtimeMetrics 注释
	KeyRealtimeMetrics = "metrics:realtime:{agent_id}"

	// KeyOnlineAgents 在线 Agent Sorted Set，score 为最后心跳时间戳
	KeyOnlineAgents = "agents:online"

	// KeyTokenBlacklist Token 黑名单，使用 Token 的 SHA256 哈希作为 key 的一部分
	KeyTokenBlacklist = "token:blacklist:{agent_id}:{token_hash}"

	// KeyRealtimeDisk 磁盘实时指标 Hash，字段为 mount_point -> JSON
	KeyRealtimeDisk = "metrics:disk:{agent_id}"

	// KeyRealtimeNetwork 网络实时指标 Hash，字段为 interface -> JSON
	KeyRealtimeNetwork = "metrics:network:{agent_id}"

	// KeyAgentDetail Agent 详情缓存，JSON 字符串
	KeyAgentDetail = "agent:detail:{agent_id}"

	// KeyAlertState 告警状态 Hash，字段为 first_trigger_at、consecutive_count、last_check_at
	KeyAlertState = "alert:state:{rule_id}:{agent_id}"
)

// RedisClient Redis 客户端封装
type RedisClient struct {
	client *redis.Client
	ctx    context.Context
}

// NewRedisClient 创建 Redis 客户端并验证连接
func NewRedisClient(addr, password string, db int) (*RedisClient, error) {
	if addr == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DB:           db,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 2,
	})

	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisClient{client: client, ctx: ctx}, nil
}

// Close 关闭 Redis 连接
func (r *RedisClient) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

// Client 返回底层 go-redis 客户端
func (r *RedisClient) Client() *redis.Client {
	return r.client
}

// Context 返回客户端使用的上下文
func (r *RedisClient) Context() context.Context {
	return r.ctx
}
