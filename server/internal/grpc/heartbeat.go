package grpc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
	pb "github.com/huang110/serverprobe/server/proto"
)

const (
	// tokenRotationThreshold Token 剩余有效期低于该值时主动轮换
	tokenRotationThreshold = 24 * time.Hour

	// configHashKey Agent 已下发配置的哈希，用于避免重复下发
	configHashKey = "agent:config:hash:{agent_id}"

	// configHashTTL 配置哈希缓存时间
	configHashTTL = 30 * 24 * time.Hour
)

// HeartbeatService 实现 Agent 心跳 RPC
type HeartbeatService struct {
	db     *gorm.DB
	redis  *cache.RedisClient
	config *config.Config
}

// NewHeartbeatService 创建心跳服务
func NewHeartbeatService(db *gorm.DB, redisClient *cache.RedisClient, cfg *config.Config) *HeartbeatService {
	return &HeartbeatService{db: db, redis: redisClient, config: cfg}
}

// Heartbeat 处理心跳：验证签名、写入指标、更新缓存和 Agent 状态。
func (s *HeartbeatService) Heartbeat(ctx context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	if req == nil {
		return &pb.HeartbeatResponse{Success: false, Message: "invalid request"}, nil
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
			return &pb.HeartbeatResponse{Success: false, Message: err.Error()}, nil
		}
		return nil, err
	}

	cpu := req.Cpu
	if cpu == nil {
		cpu = &pb.CPUMetrics{}
	}
	memory := req.Memory
	if memory == nil {
		memory = &pb.MemoryMetrics{}
	}
	system := req.System
	if system == nil {
		system = &pb.SystemMetrics{}
	}

	var totalRxBytes, totalTxBytes int64
	for _, network := range req.Networks {
		if network == nil {
			continue
		}
		totalRxBytes += network.RxBytesPerSec
		totalTxBytes += network.TxBytesPerSec
	}

	// 使用请求时间戳选择分区，避免月初边界写入错误分区
	tableName := model.GetMetricsRawTableName(time.Unix(req.Timestamp, 0))
	metricsRaw := model.MetricsRaw{
		AgentID:       req.AgentId,
		Timestamp:     req.Timestamp,
		CPUUsage:      cpu.UsagePercent,
		CPUUser:       cpu.UserPercent,
		CPUSystem:     cpu.SystemPercent,
		MemoryUsed:    memory.Used,
		MemoryPercent: memory.UsagePercent,
		SwapUsed:      memory.SwapUsed,
		SwapPercent:   memory.SwapUsagePercent,
		Load1:         system.Load1,
		Load5:         system.Load5,
		Load15:        system.Load15,
		NetRxBytes:    totalRxBytes,
		NetTxBytes:    totalTxBytes,
		TCPEstab:      system.TcpEstablished,
		TCPTimeWait:   system.TcpTimeWait,
	}
	if err := s.db.WithContext(ctx).Table(tableName).Create(&metricsRaw).Error; err != nil {
		return nil, fmt.Errorf("failed to save metrics: %w", err)
	}

	s.saveDiskMetrics(ctx, req)
	s.saveNetworkMetrics(ctx, req)
	s.updateRealtimeCache(ctx, req, totalRxBytes, totalTxBytes)

	// Agent 状态更新失败不影响本次心跳
	if err := s.db.WithContext(ctx).Model(&model.Agent{}).
		Where("id = ?", req.AgentId).
		Updates(map[string]interface{}{
			"last_heartbeat_at": req.Timestamp,
			"status":            agentStatusOnline,
		}).Error; err != nil {
		log.Printf("warning: failed to update agent status for %s: %v", req.AgentId, err)
	}

	response := &pb.HeartbeatResponse{
		Success:               true,
		NextHeartbeatInterval: s.config.GetHeartbeatInterval(),
	}

	rotated, rotatedToken, rotatedExpiresAt, err := s.rotateTokenIfNeeded(ctx, agent)
	if err != nil {
		return nil, err
	}
	if rotated {
		response.NewToken = rotatedToken
		response.NewTokenExpiresAt = rotatedExpiresAt
	}

	s.attachConfigUpdate(ctx, req.AgentId, response)

	return response, nil
}

// saveDiskMetrics 保存磁盘明细，失败仅记录日志
func (s *HeartbeatService) saveDiskMetrics(ctx context.Context, req *pb.HeartbeatRequest) {
	for _, disk := range req.Disks {
		if disk == nil {
			continue
		}
		metricsDisk := model.MetricsDisk{
			AgentID:      req.AgentId,
			MountPoint:   disk.MountPoint,
			Timestamp:    req.Timestamp,
			Device:       disk.Device,
			FsType:       disk.FsType,
			Total:        disk.Total,
			Used:         disk.Used,
			UsagePercent: disk.UsagePercent,
			InodePercent: disk.InodeUsagePercent,
			ReadBytes:    disk.ReadBytesPerSec,
			WriteBytes:   disk.WriteBytesPerSec,
			IOUtil:       disk.IoUtilPercent,
		}
		if err := s.db.WithContext(ctx).Create(&metricsDisk).Error; err != nil {
			log.Printf("warning: failed to save disk metrics for %s: %v", disk.MountPoint, err)
		}
	}
}

// saveNetworkMetrics 保存网络明细，失败仅记录日志
func (s *HeartbeatService) saveNetworkMetrics(ctx context.Context, req *pb.HeartbeatRequest) {
	for _, network := range req.Networks {
		if network == nil {
			continue
		}
		metricsNetwork := model.MetricsNetwork{
			AgentID:   req.AgentId,
			Interface: network.Interface,
			Timestamp: req.Timestamp,
			IPAddress: network.IpAddress,
			RxBytes:   network.RxBytesPerSec,
			TxBytes:   network.TxBytesPerSec,
			RxPackets: network.RxPackets,
			TxPackets: network.TxPackets,
			RxErrors:  network.RxErrors,
			TxErrors:  network.TxErrors,
		}
		if err := s.db.WithContext(ctx).Create(&metricsNetwork).Error; err != nil {
			log.Printf("warning: failed to save network metrics for %s: %v", network.Interface, err)
		}
	}
}

// updateRealtimeCache 更新 Redis 实时缓存，失败不影响心跳
func (s *HeartbeatService) updateRealtimeCache(ctx context.Context, req *pb.HeartbeatRequest, totalRxBytes, totalTxBytes int64) {
	if s.redis == nil {
		return
	}

	cpu := req.Cpu
	if cpu == nil {
		cpu = &pb.CPUMetrics{}
	}
	memory := req.Memory
	if memory == nil {
		memory = &pb.MemoryMetrics{}
	}
	system := req.System
	if system == nil {
		system = &pb.SystemMetrics{}
	}

	realtimeMetrics := map[string]interface{}{
		"cpu_usage":         cpu.UsagePercent,
		"memory_used":       memory.Used,
		"memory_percent":    memory.UsagePercent,
		"swap_percent":      memory.SwapUsagePercent,
		"load1":             system.Load1,
		"load5":             system.Load5,
		"load15":            system.Load15,
		"net_rx_bytes":      totalRxBytes,
		"net_tx_bytes":      totalTxBytes,
		"tcp_established":   system.TcpEstablished,
		"tcp_time_wait":     system.TcpTimeWait,
		"status":            agentStatusOnline,
		"last_heartbeat_at": req.Timestamp,
	}
	if err := s.redis.SaveRealtimeMetrics(req.AgentId, realtimeMetrics); err != nil {
		log.Printf("warning: failed to cache realtime metrics for %s: %v", req.AgentId, err)
	}

	diskMetrics := make(map[string]interface{}, len(req.Disks))
	for _, disk := range req.Disks {
		if disk == nil {
			continue
		}
		diskMetrics[disk.MountPoint] = map[string]interface{}{
			"usage_percent": disk.UsagePercent,
			"read_bytes":    disk.ReadBytesPerSec,
			"write_bytes":   disk.WriteBytesPerSec,
			"io_util":       disk.IoUtilPercent,
		}
	}
	if err := s.redis.SaveDiskMetrics(req.AgentId, diskMetrics); err != nil {
		log.Printf("warning: failed to cache disk metrics for %s: %v", req.AgentId, err)
	}

	networkMetrics := make(map[string]interface{}, len(req.Networks))
	for _, network := range req.Networks {
		if network == nil {
			continue
		}
		networkMetrics[network.Interface] = map[string]interface{}{
			"rx_bytes":  network.RxBytesPerSec,
			"tx_bytes":  network.TxBytesPerSec,
			"rx_errors": network.RxErrors,
			"tx_errors": network.TxErrors,
		}
	}
	if err := s.redis.SaveNetworkMetrics(req.AgentId, networkMetrics); err != nil {
		log.Printf("warning: failed to cache network metrics for %s: %v", req.AgentId, err)
	}

	if err := s.redis.UpdateOnlineAgent(req.AgentId, req.Timestamp); err != nil {
		log.Printf("warning: failed to update online agents for %s: %v", req.AgentId, err)
	}

	if err := s.redis.PublishEvent(ctx, cache.AgentEvent{
		Type:      cache.EventTypeHeartbeat,
		AgentID:   req.AgentId,
		Timestamp: req.Timestamp,
	}); err != nil {
		log.Printf("warning: failed to publish heartbeat event for %s: %v", req.AgentId, err)
	}
}

// rotateTokenIfNeeded Token 剩余有效期小于 24 小时时主动轮换
func (s *HeartbeatService) rotateTokenIfNeeded(ctx context.Context, agent *model.Agent) (bool, string, int64, error) {
	if agent == nil || agent.TokenExpiresAt <= 0 {
		return false, "", 0, nil
	}

	now := time.Now().Unix()
	if agent.TokenExpiresAt-now >= int64(tokenRotationThreshold/time.Second) {
		return false, "", 0, nil
	}

	newToken, err := generateToken()
	if err != nil {
		return false, "", 0, err
	}
	newExpiresAt := tokenExpiresAt(s.config, now)

	// 先更新数据库，再吊销旧 Token；数据库失败时不改变任何 Token 状态
	if err := s.db.WithContext(ctx).Model(&model.Agent{}).
		Where("id = ?", agent.ID).
		Updates(map[string]interface{}{
			"token":            newToken,
			"token_expires_at": newExpiresAt,
		}).Error; err != nil {
		return false, "", 0, fmt.Errorf("failed to update agent token: %w", err)
	}

	if s.redis != nil {
		if err := s.redis.AddTokenToBlacklist(agent.ID, agent.Token, 10*time.Minute); err != nil {
			log.Printf("warning: failed to blacklist old token for %s: %v", agent.ID, err)
		}
	}

	agent.Token = newToken
	agent.TokenExpiresAt = newExpiresAt

	return true, newToken, newExpiresAt, nil
}

// attachConfigUpdate 在配置发生变化时向 Agent 下发配置
func (s *HeartbeatService) attachConfigUpdate(ctx context.Context, agentID string, response *pb.HeartbeatResponse) {
	labels, err := loadAgentLabels(ctx, s.db, agentID)
	if err != nil {
		log.Printf("warning: failed to load labels for %s: %v", agentID, err)
		return
	}

	if !s.shouldSendConfigUpdate(ctx, agentID, labels) {
		return
	}

	response.ConfigUpdate = &pb.ConfigUpdate{
		HasUpdate:         true,
		HeartbeatInterval: s.config.GetHeartbeatInterval(),
		Labels:            labels,
		EnableIpQuality:   s.config.EnableIPQuality,
	}
}

// loadAgentLabels 查询 Agent 标签
func loadAgentLabels(ctx context.Context, db *gorm.DB, agentID string) (map[string]string, error) {
	var labels []model.AgentLabel
	if err := db.WithContext(ctx).Where("agent_id = ?", agentID).Find(&labels).Error; err != nil {
		return nil, err
	}

	result := make(map[string]string, len(labels))
	for _, label := range labels {
		result[label.Key] = label.Value
	}
	return result, nil
}

// shouldSendConfigUpdate 通过配置哈希避免每次心跳重复下发相同配置
func (s *HeartbeatService) shouldSendConfigUpdate(ctx context.Context, agentID string, labels map[string]string) bool {
	hash := buildConfigHash(s.config.GetHeartbeatInterval(), s.config.EnableIPQuality, labels)
	if s.redis == nil {
		return true
	}

	key := strings.ReplaceAll(configHashKey, "{agent_id}", agentID)
	previous, err := s.redis.Client().Get(ctx, key).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		log.Printf("warning: failed to read config hash for %s: %v", agentID, err)
		return false
	}
	if previous == hash {
		return false
	}

	if err := s.redis.Client().Set(ctx, key, hash, configHashTTL).Err(); err != nil {
		log.Printf("warning: failed to save config hash for %s: %v", agentID, err)
	}
	return true
}

// buildConfigHash 生成稳定的配置哈希
func buildConfigHash(interval int32, enableIPQuality bool, labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%d|%t|", interval, enableIPQuality)
	for _, key := range keys {
		_, _ = fmt.Fprintf(hash, "%s=%s|", key, labels[key])
	}
	return hex.EncodeToString(hash.Sum(nil))
}
