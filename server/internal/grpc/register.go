package grpc

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
	pb "github.com/huang110/serverprobe/server/proto"
)

const (
	// tokenLength Token 随机字节数（hex 编码后长度为 64）
	tokenLength = 32

	// agentStatusOnline Agent 在线状态
	agentStatusOnline = "online"
)

// RegisterService 实现 Agent 注册 RPC
type RegisterService struct {
	db     *gorm.DB
	config *config.Config
}

// NewRegisterService 创建注册服务
func NewRegisterService(db *gorm.DB, cfg *config.Config) *RegisterService {
	return &RegisterService{db: db, config: cfg}
}

// Register 处理 Agent 注册请求。
// 相同 hostname 的 Agent 会复用原 agent_id；Token 过期时自动续期。
func (s *RegisterService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req == nil {
		return nil, errors.New("register request is required")
	}
	if s.db == nil {
		return nil, errors.New("database is required")
	}
	if s.config == nil {
		return nil, errors.New("config is required")
	}

	if !secureCompare(req.RegisterSecret, s.config.GetRegisterSecret()) {
		return &pb.RegisterResponse{
			Success: false,
			Message: "invalid register secret",
		}, nil
	}

	hostname := strings.TrimSpace(req.Hostname)
	if hostname == "" {
		return &pb.RegisterResponse{
			Success: false,
			Message: "hostname is required",
		}, nil
	}

	now := time.Now().Unix()

	var existing model.Agent
	err := s.db.WithContext(ctx).Where("hostname = ?", hostname).First(&existing).Error
	switch {
	case err == nil:
		return s.renewOrReturnExisting(ctx, &existing, req, hostname, now)
	case errors.Is(err, gorm.ErrRecordNotFound):
		return s.createAgent(ctx, req, hostname, now)
	default:
		return nil, fmt.Errorf("failed to query agent by hostname: %w", err)
	}
}

// createAgent 创建新 Agent
func (s *RegisterService) createAgent(ctx context.Context, req *pb.RegisterRequest, hostname string, now int64) (*pb.RegisterResponse, error) {
	agentID := uuid.NewString()
	token, err := generateToken()
	if err != nil {
		return nil, err
	}

	expiresAt := tokenExpiresAt(s.config, now)
	agent := model.Agent{
		ID:              agentID,
		Hostname:        hostname,
		IP:              req.Ip,
		InternalIP:      req.InternalIp,
		OS:              req.Os,
		OSVersion:       req.OsVersion,
		KernelVersion:   req.KernelVersion,
		CPUCores:        req.CpuCores,
		MemoryTotal:     req.MemoryTotal,
		DiskTotal:       req.DiskTotal,
		Arch:            req.Arch,
		AgentVersion:    req.AgentVersion,
		Token:           token,
		TokenExpiresAt:  expiresAt,
		LastHeartbeatAt: now,
		Status:          agentStatusOnline,
		LiteMode:        req.LiteMode,
	}

	if err := s.db.WithContext(ctx).Create(&agent).Error; err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	if err := s.saveLabels(ctx, agentID, req.Labels); err != nil {
		// 标签保存失败不应导致注册失败
		log.Printf("warning: failed to save labels for agent %s: %v", agentID, err)
	}

	return &pb.RegisterResponse{
		Success:           true,
		Message:           "registration successful",
		AgentId:           agent.ID,
		Token:             agent.Token,
		HeartbeatInterval: s.config.GetHeartbeatInterval(),
		TokenExpiresAt:    agent.TokenExpiresAt,
	}, nil
}

// renewOrReturnExisting 处理已存在的 Agent。
// Token 未过期时返回原凭据，同时刷新系统信息和标签。
func (s *RegisterService) renewOrReturnExisting(ctx context.Context, existing *model.Agent, req *pb.RegisterRequest, hostname string, now int64) (*pb.RegisterResponse, error) {
	message := "agent already registered"
	token := existing.Token

	if token == "" || existing.TokenExpiresAt <= now {
		generated, err := generateToken()
		if err != nil {
			return nil, err
		}
		token = generated
		existing.TokenExpiresAt = tokenExpiresAt(s.config, now)
		message = "token renewed"
	}

	existing.Hostname = hostname
	existing.IP = req.Ip
	existing.InternalIP = req.InternalIp
	existing.OS = req.Os
	existing.OSVersion = req.OsVersion
	existing.KernelVersion = req.KernelVersion
	existing.CPUCores = req.CpuCores
	existing.MemoryTotal = req.MemoryTotal
	existing.DiskTotal = req.DiskTotal
	existing.Arch = req.Arch
	existing.AgentVersion = req.AgentVersion
	existing.Token = token
	existing.LastHeartbeatAt = now
	existing.Status = agentStatusOnline
	existing.LiteMode = req.LiteMode

	if err := s.db.WithContext(ctx).Save(existing).Error; err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	if err := s.saveLabels(ctx, existing.ID, req.Labels); err != nil {
		log.Printf("warning: failed to save labels for agent %s: %v", existing.ID, err)
	}

	return &pb.RegisterResponse{
		Success:           true,
		Message:           message,
		AgentId:           existing.ID,
		Token:             existing.Token,
		HeartbeatInterval: s.config.GetHeartbeatInterval(),
		TokenExpiresAt:    existing.TokenExpiresAt,
	}, nil
}

// saveLabels 原子替换 Agent 的全部标签
func (s *RegisterService) saveLabels(ctx context.Context, agentID string, labels map[string]string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("agent_id = ?", agentID).Delete(&model.AgentLabel{}).Error; err != nil {
			return fmt.Errorf("failed to delete old labels: %w", err)
		}

		if len(labels) == 0 {
			return nil
		}

		records := make([]model.AgentLabel, 0, len(labels))
		normalized := make(map[string]string, len(labels))
		for key, value := range labels {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			normalized[key] = value
		}
		for key, value := range normalized {
			records = append(records, model.AgentLabel{
				AgentID: agentID,
				Key:     key,
				Value:   value,
			})
		}
		if len(records) == 0 {
			return nil
		}

		if err := tx.Create(&records).Error; err != nil {
			return fmt.Errorf("failed to insert labels: %w", err)
		}
		return nil
	})
}

// generateToken 使用 crypto/rand 生成 32 字节随机 Token
func generateToken() (string, error) {
	buffer := make([]byte, tokenLength)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

// tokenExpiresAt 计算 Token 过期时间
func tokenExpiresAt(cfg *config.Config, now int64) int64 {
	days := cfg.GetTokenExpirationDays()
	return time.Unix(now, 0).AddDate(0, 0, days).Unix()
}

// secureCompare 使用常量时间比较注册密钥
func secureCompare(actual, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
