package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// tokenBlacklistTTL Token 黑名单默认 TTL，等于 2 倍签名时间窗口
	tokenBlacklistTTL = 10 * time.Minute
)

// AddTokenToBlacklist 将旧 Token 加入黑名单。
// ttl <= 0 时使用默认的 10 分钟。
func (r *RedisClient) AddTokenToBlacklist(agentID, token string, ttl time.Duration) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}
	if token == "" {
		return errors.New("token is required")
	}
	if ttl <= 0 {
		ttl = tokenBlacklistTTL
	}

	key := tokenBlacklistKey(agentID, token)
	if err := r.client.Set(r.ctx, key, "1", ttl).Err(); err != nil {
		return fmt.Errorf("failed to add token to blacklist: %w", err)
	}
	return nil
}

// IsTokenBlacklisted 检查 Token 是否已在黑名单中
func (r *RedisClient) IsTokenBlacklisted(agentID, token string) (bool, error) {
	if agentID == "" {
		return false, errors.New("agent id is required")
	}
	if token == "" {
		return false, nil
	}

	exists, err := r.client.Exists(r.ctx, tokenBlacklistKey(agentID, token)).Result()
	if err != nil {
		return false, fmt.Errorf("failed to check token blacklist: %w", err)
	}
	return exists > 0, nil
}

// RemoveTokenFromBlacklist 从黑名单移除 Token（通常不需要，保留用于运维）
func (r *RedisClient) RemoveTokenFromBlacklist(agentID, token string) error {
	if agentID == "" {
		return errors.New("agent id is required")
	}
	if token == "" {
		return nil
	}
	return r.client.Del(r.ctx, tokenBlacklistKey(agentID, token)).Err()
}

// hashToken 计算 Token 的 SHA256 哈希，避免在 Redis 中存储明文 Token
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func tokenBlacklistKey(agentID, token string) string {
	key := strings.ReplaceAll(KeyTokenBlacklist, "{agent_id}", agentID)
	return strings.ReplaceAll(key, "{token_hash}", hashToken(token))
}
