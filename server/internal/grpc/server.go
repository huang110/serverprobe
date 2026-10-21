package grpc

import (
	"context"

	ggrpc "google.golang.org/grpc"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	pb "github.com/huang110/serverprobe/server/proto"
)

// ProbeService 组合全部 gRPC RPC 实现
type ProbeService struct {
	pb.UnimplementedProbeServiceServer

	register  *RegisterService
	heartbeat *HeartbeatService
	config    *ConfigService
	token     *TokenService
}

var _ pb.ProbeServiceServer = (*ProbeService)(nil)

// NewProbeService 创建组合服务
func NewProbeService(db *gorm.DB, redisClient *cache.RedisClient, cfg *config.Config) *ProbeService {
	return &ProbeService{
		register:  NewRegisterService(db, cfg),
		heartbeat: NewHeartbeatService(db, redisClient, cfg),
		config:    NewConfigService(db, redisClient, cfg),
		token:     NewTokenService(db, redisClient, cfg),
	}
}

// RegisterProbeServer 将 ProbeService 注册到 gRPC Server
func RegisterProbeServer(server *ggrpc.Server, db *gorm.DB, redisClient *cache.RedisClient, cfg *config.Config) *ProbeService {
	service := NewProbeService(db, redisClient, cfg)
	pb.RegisterProbeServiceServer(server, service)
	return service
}

// Register 注册 RPC
func (s *ProbeService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return s.register.Register(ctx, req)
}

// Heartbeat 心跳 RPC
func (s *ProbeService) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	return s.heartbeat.Heartbeat(ctx, req)
}

// GetConfig 配置拉取 RPC
func (s *ProbeService) GetConfig(ctx context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	return s.config.GetConfig(ctx, req)
}

// RotateToken Token 轮换 RPC
func (s *ProbeService) RotateToken(ctx context.Context, req *pb.RotateTokenRequest) (*pb.RotateTokenResponse, error) {
	return s.token.RotateToken(ctx, req)
}
