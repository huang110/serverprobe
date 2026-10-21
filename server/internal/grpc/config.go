package grpc

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	pb "github.com/huang110/serverprobe/server/proto"
)

// ConfigService 实现 Agent 配置拉取 RPC
type ConfigService struct {
	db     *gorm.DB
	redis  *cache.RedisClient
	config *config.Config
}

// NewConfigService 创建配置拉取服务
func NewConfigService(db *gorm.DB, redisClient *cache.RedisClient, cfg *config.Config) *ConfigService {
	return &ConfigService{db: db, redis: redisClient, config: cfg}
}

// GetConfig 返回心跳间隔、Agent 标签和功能开关。
func (s *ConfigService) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	if req == nil {
		return &pb.GetConfigResponse{Success: false}, nil
	}
	if s.db == nil {
		return nil, errors.New("database is required")
	}
	if s.config == nil {
		return nil, errors.New("config is required")
	}

	if _, err := verifyRequest(ctx, s.db, s.redis, req.AgentId, req.Timestamp, req.Signature); err != nil {
		if isVerificationError(err) {
			return &pb.GetConfigResponse{Success: false}, nil
		}
		return nil, err
	}

	labels, err := loadAgentLabels(ctx, s.db, req.AgentId)
	if err != nil {
		return nil, err
	}

	return &pb.GetConfigResponse{
		Success:           true,
		HeartbeatInterval: s.config.GetHeartbeatInterval(),
		Labels:            labels,
		EnableIpQuality:   s.config.EnableIPQuality,
	}, nil
}
