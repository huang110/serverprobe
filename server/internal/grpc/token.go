package grpc

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
	pb "github.com/huang110/serverprobe/server/proto"
)

// TokenService 实现 Agent Token 轮换 RPC
type TokenService struct {
	db     *gorm.DB
	redis  *cache.RedisClient
	config *config.Config
}

// NewTokenService 创建 Token 轮换服务
func NewTokenService(db *gorm.DB, redisClient *cache.RedisClient, cfg *config.Config) *TokenService {
	return &TokenService{db: db, redis: redisClient, config: cfg}
}

// RotateToken 验证当前 Token 后签发新 Token，并将旧 Token 加入黑名单。
func (s *TokenService) RotateToken(ctx context.Context, req *pb.RotateTokenRequest) (*pb.RotateTokenResponse, error) {
	if req == nil {
		return &pb.RotateTokenResponse{Success: false}, nil
	}
	if s.db == nil {
		return nil, errors.New("database is required")
	}
	if s.config == nil {
		return nil, errors.New("config is required")
	}

	agent, err := verifyRequest(ctx, s.db, s.redis, req.AgentId, req.Timestamp, req.Signature)
	if err != nil {
		if isVerificationError(err) {
			return &pb.RotateTokenResponse{Success: false}, nil
		}
		return nil, err
	}

	newToken, err := generateToken()
	if err != nil {
		return nil, err
	}
	newExpiresAt := tokenExpiresAt(s.config, time.Now().Unix())

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&model.Agent{}).
			Where("id = ?", agent.ID).
			Updates(map[string]interface{}{
				"token":            newToken,
				"token_expires_at": newExpiresAt,
			})
		if result.Error != nil {
			return fmt.Errorf("failed to update token: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return errAgentNotFound
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 数据库更新成功后再吊销旧 Token；黑名单失败不影响新 Token 生效
	if s.redis != nil {
		if err := s.redis.AddTokenToBlacklist(agent.ID, agent.Token, 10*time.Minute); err != nil {
			log.Printf("warning: failed to blacklist old token for %s: %v", agent.ID, err)
		}
	}

	return &pb.RotateTokenResponse{
		Success:   true,
		NewToken:  newToken,
		ExpiresAt: newExpiresAt,
	}, nil
}
