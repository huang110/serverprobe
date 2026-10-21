package grpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/model"
	"github.com/huang110/serverprobe/server/internal/pkg/auth"
)

var (
	// errAgentNotFound Agent 不存在
	errAgentNotFound = errors.New("agent not found")

	// errTimestampExpired 请求时间戳超出允许窗口
	errTimestampExpired = errors.New("timestamp expired")

	// errTokenRevoked Token 已被吊销
	errTokenRevoked = errors.New("token has been revoked")

	// errInvalidSignature HMAC 签名无效
	errInvalidSignature = errors.New("invalid signature")

	// errTokenExpired Agent Token 已过期
	errTokenExpired = errors.New("token expired")
)

// verifyRequest 校验 Agent 身份和请求签名。
// Redis 黑名单查询失败时记录日志但继续验证签名，避免缓存故障导致全部 Agent 掉线。
func verifyRequest(
	ctx context.Context,
	db *gorm.DB,
	redis *cache.RedisClient,
	agentID string,
	timestamp int64,
	signature string,
) (*model.Agent, error) {
	if db == nil {
		return nil, errors.New("database is required")
	}

	var agent model.Agent
	if err := db.WithContext(ctx).First(&agent, "id = ?", agentID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errAgentNotFound
		}
		return nil, fmt.Errorf("failed to query agent: %w", err)
	}

	if !auth.ValidateTimestamp(timestamp) {
		return nil, errTimestampExpired
	}

	if redis != nil {
		blacklisted, err := redis.IsTokenBlacklisted(agentID, agent.Token)
		if err != nil {
			log.Printf("warning: failed to check token blacklist for agent %s: %v", agentID, err)
		} else if blacklisted {
			return nil, errTokenRevoked
		}
	}

	if agent.Token == "" {
		return nil, errInvalidSignature
	}
	if !auth.VerifySignature(agentID, timestamp, agent.Token, signature) {
		return nil, errInvalidSignature
	}

	if agent.TokenExpiresAt > 0 && agent.TokenExpiresAt <= time.Now().Unix() {
		return nil, errTokenExpired
	}

	return &agent, nil
}

// isVerificationError 判断错误是否属于可返回给 Agent 的鉴权/业务错误
func isVerificationError(err error) bool {
	return errors.Is(err, errAgentNotFound) ||
		errors.Is(err, errTimestampExpired) ||
		errors.Is(err, errTokenRevoked) ||
		errors.Is(err, errInvalidSignature) ||
		errors.Is(err, errTokenExpired)
}
