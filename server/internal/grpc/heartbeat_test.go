package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"gorm.io/gorm"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
	"github.com/huang110/serverprobe/server/internal/pkg/auth"
	pb "github.com/huang110/serverprobe/server/proto"
)

// setupTestRedis 启动 miniredis 并创建缓存客户端
func setupTestRedis(t *testing.T) *cache.RedisClient {
	t.Helper()

	server := miniredis.RunT(t)
	client, err := cache.NewRedisClient(server.Addr(), "", 0)
	if err != nil {
		t.Fatalf("NewRedisClient() failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("Close() failed: %v", closeErr)
		}
	})
	return client
}

// createTestAgent 创建测试 Agent
func createTestAgent(t *testing.T, db *gorm.DB, id, token string, expiresAt int64) *model.Agent {
	t.Helper()

	agent := &model.Agent{
		ID:              id,
		Hostname:        "node-" + id,
		Token:           token,
		TokenExpiresAt:  expiresAt,
		LastHeartbeatAt: time.Now().Unix(),
		Status:          agentStatusOnline,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	return agent
}

// buildHeartbeatRequest 构造签名正确的心跳请求
func buildHeartbeatRequest(agent *model.Agent, timestamp int64) *pb.HeartbeatRequest {
	return &pb.HeartbeatRequest{
		AgentId:   agent.ID,
		Timestamp: timestamp,
		Signature: auth.GenerateSignature(agent.ID, timestamp, agent.Token),
		Cpu:       &pb.CPUMetrics{UsagePercent: 25.5, UserPercent: 10, SystemPercent: 5},
		Memory: &pb.MemoryMetrics{
			Used:             2048,
			UsagePercent:     50,
			SwapUsed:         128,
			SwapUsagePercent: 5,
		},
		System: &pb.SystemMetrics{
			Load1:          1.5,
			Load5:          1.2,
			Load15:         1.0,
			TcpEstablished: 10,
			TcpTimeWait:    3,
		},
		Disks: []*pb.DiskMetrics{{
			Device:            "/dev/sda1",
			MountPoint:        "/",
			FsType:            "ext4",
			Total:             100000,
			Used:              60000,
			UsagePercent:      60,
			InodeUsagePercent: 20,
			ReadBytesPerSec:   1024,
			WriteBytesPerSec:  2048,
			IoUtilPercent:     10,
		}},
		Networks: []*pb.NetworkMetrics{{
			Interface:     "eth0",
			IpAddress:     "10.0.0.5",
			RxBytesPerSec: 100,
			TxBytesPerSec: 50,
			RxPackets:     10,
			TxPackets:     5,
			RxErrors:      1,
			TxErrors:      0,
		}},
	}
}

func TestHeartbeatSuccess(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	cfg := &config.Config{
		HeartbeatInterval:   30,
		TokenExpirationDays: 7,
		EnableIPQuality:     true,
	}
	svc := NewHeartbeatService(db, redisClient, cfg)

	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())
	if err := db.Create(&model.AgentLabel{AgentID: agent.ID, Key: "env", Value: "prod"}).Error; err != nil {
		t.Fatalf("failed to create label: %v", err)
	}

	timestamp := time.Now().Unix()
	req := buildHeartbeatRequest(agent, timestamp)

	resp, err := svc.Heartbeat(context.Background(), req)
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Heartbeat() failed: %s", resp.Message)
	}
	if resp.NextHeartbeatInterval != 30 {
		t.Errorf("NextHeartbeatInterval = %d, want 30", resp.NextHeartbeatInterval)
	}
	if resp.ConfigUpdate == nil || !resp.ConfigUpdate.HasUpdate {
		t.Error("first heartbeat should include config update")
	} else if resp.ConfigUpdate.Labels["env"] != "prod" {
		t.Errorf("config update labels = %+v, want env=prod", resp.ConfigUpdate.Labels)
	}

	tableName := model.GetMetricsRawTableName(time.Unix(timestamp, 0))
	var raw model.MetricsRaw
	if err := db.Table(tableName).Where("agent_id = ?", agent.ID).First(&raw).Error; err != nil {
		t.Fatalf("metrics_raw not found: %v", err)
	}
	if raw.CPUUsage != 25.5 || raw.MemoryUsed != 2048 || raw.Load1 != 1.5 {
		t.Errorf("unexpected raw metrics: %+v", raw)
	}
	if raw.NetRxBytes != 100 || raw.NetTxBytes != 50 || raw.TCPEstab != 10 {
		t.Errorf("unexpected network/connection metrics: %+v", raw)
	}

	var disk model.MetricsDisk
	if err := db.Where("agent_id = ?", agent.ID).First(&disk).Error; err != nil {
		t.Fatalf("metrics_disk not found: %v", err)
	}
	if disk.MountPoint != "/" || disk.UsagePercent != 60 || disk.InodePercent != 20 {
		t.Errorf("unexpected disk metrics: %+v", disk)
	}

	var network model.MetricsNetwork
	if err := db.Where("agent_id = ?", agent.ID).First(&network).Error; err != nil {
		t.Fatalf("metrics_network not found: %v", err)
	}
	if network.Interface != "eth0" || network.RxBytes != 100 || network.TxBytes != 50 {
		t.Errorf("unexpected network metrics: %+v", network)
	}

	realtime, err := redisClient.GetRealtimeMetrics(agent.ID)
	if err != nil {
		t.Fatalf("GetRealtimeMetrics() failed: %v", err)
	}
	if realtime["cpu_usage"] != "25.5" {
		t.Errorf("cached cpu_usage = %q, want %q", realtime["cpu_usage"], "25.5")
	}
	online, err := redisClient.GetOnlineAgents(300)
	if err != nil {
		t.Fatalf("GetOnlineAgents() failed: %v", err)
	}
	if len(online) != 1 || online[0] != agent.ID {
		t.Errorf("online agents = %v, want [%s]", online, agent.ID)
	}

	var updated model.Agent
	if err := db.First(&updated, "id = ?", agent.ID).Error; err != nil {
		t.Fatalf("failed to query updated agent: %v", err)
	}
	if updated.Status != agentStatusOnline || updated.LastHeartbeatAt != timestamp {
		t.Errorf("agent status not updated: %+v", updated)
	}

	// 配置未变化时第二次心跳不应重复下发
	second, err := svc.Heartbeat(context.Background(), buildHeartbeatRequest(agent, time.Now().Unix()))
	if err != nil {
		t.Fatalf("second Heartbeat() returned error: %v", err)
	}
	if second.ConfigUpdate != nil && second.ConfigUpdate.HasUpdate {
		t.Error("unchanged config should not be pushed again")
	}
}

func TestHeartbeatInvalidSignature(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewHeartbeatService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())

	req := buildHeartbeatRequest(agent, time.Now().Unix())
	req.Signature = "invalid-signature"

	resp, err := svc.Heartbeat(context.Background(), req)
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if resp.Success {
		t.Error("Heartbeat() should reject invalid signature")
	}
	if resp.Message != "invalid signature" {
		t.Errorf("Message = %q, want %q", resp.Message, "invalid signature")
	}
}

func TestHeartbeatExpiredTimestamp(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewHeartbeatService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())

	expiredTimestamp := time.Now().Add(-10 * time.Minute).Unix()
	req := buildHeartbeatRequest(agent, expiredTimestamp)

	resp, err := svc.Heartbeat(context.Background(), req)
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if resp.Success {
		t.Error("Heartbeat() should reject expired timestamp")
	}
	if resp.Message != "timestamp expired" {
		t.Errorf("Message = %q, want %q", resp.Message, "timestamp expired")
	}
}

func TestHeartbeatAgentNotFound(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewHeartbeatService(db, redisClient, &config.Config{HeartbeatInterval: 30})

	resp, err := svc.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		AgentId:   "missing",
		Timestamp: time.Now().Unix(),
		Signature: "whatever",
	})
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if resp.Success || resp.Message != "agent not found" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHeartbeatRevokedToken(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewHeartbeatService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())

	if err := redisClient.AddTokenToBlacklist(agent.ID, agent.Token, time.Minute); err != nil {
		t.Fatalf("failed to blacklist token: %v", err)
	}

	resp, err := svc.Heartbeat(context.Background(), buildHeartbeatRequest(agent, time.Now().Unix()))
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if resp.Success || resp.Message != "token has been revoked" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHeartbeatTokenRotation(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	cfg := &config.Config{HeartbeatInterval: 30, TokenExpirationDays: 7}
	svc := NewHeartbeatService(db, redisClient, cfg)

	agent := createTestAgent(t, db, "agent-1", "old-token", time.Now().Add(12*time.Hour).Unix())

	resp, err := svc.Heartbeat(context.Background(), buildHeartbeatRequest(agent, time.Now().Unix()))
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Heartbeat() failed: %s", resp.Message)
	}
	if resp.NewToken == "" || resp.NewToken == "old-token" {
		t.Fatal("NewToken should be generated and different from old token")
	}
	if resp.NewTokenExpiresAt <= time.Now().Unix() {
		t.Error("NewTokenExpiresAt should be in the future")
	}

	blacklisted, err := redisClient.IsTokenBlacklisted(agent.ID, "old-token")
	if err != nil {
		t.Fatalf("IsTokenBlacklisted() failed: %v", err)
	}
	if !blacklisted {
		t.Error("old token should be blacklisted")
	}

	var updated model.Agent
	if err := db.First(&updated, "id = ?", agent.ID).Error; err != nil {
		t.Fatalf("failed to query agent: %v", err)
	}
	if updated.Token != resp.NewToken || updated.TokenExpiresAt != resp.NewTokenExpiresAt {
		t.Errorf("agent token not updated: %+v", updated)
	}

	// 旧 Token 不应再能通过验证
	oldResp, err := svc.Heartbeat(context.Background(), buildHeartbeatRequest(agent, time.Now().Unix()))
	if err != nil {
		t.Fatalf("old-token Heartbeat() returned error: %v", err)
	}
	if oldResp.Success {
		t.Error("old token should be rejected after rotation")
	}

	// 新 Token 可以正常心跳
	agent.Token = resp.NewToken
	newResp, err := svc.Heartbeat(context.Background(), buildHeartbeatRequest(agent, time.Now().Unix()))
	if err != nil {
		t.Fatalf("new-token Heartbeat() returned error: %v", err)
	}
	if !newResp.Success {
		t.Errorf("new token should be accepted: %s", newResp.Message)
	}
}

func TestHeartbeatTokenExpired(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewHeartbeatService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(-time.Hour).Unix())

	resp, err := svc.Heartbeat(context.Background(), buildHeartbeatRequest(agent, time.Now().Unix()))
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if resp.Success || resp.Message != "token expired" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestHeartbeatNilRequest(t *testing.T) {
	svc := NewHeartbeatService(nil, nil, nil)
	resp, err := svc.Heartbeat(context.Background(), nil)
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if resp.Success || resp.Message != "invalid request" {
		t.Errorf("unexpected response: %+v", resp)
	}
}

func TestBuildConfigHash(t *testing.T) {
	first := buildConfigHash(30, true, map[string]string{"env": "prod", "role": "api"})
	second := buildConfigHash(30, true, map[string]string{"role": "api", "env": "prod"})
	if first != second {
		t.Error("config hash should be independent of map iteration order")
	}
	if first == buildConfigHash(60, true, map[string]string{"env": "prod", "role": "api"}) {
		t.Error("config hash should change when interval changes")
	}
}

func TestHeartbeatMissingMetricsAreSafe(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewHeartbeatService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())

	timestamp := time.Now().Unix()
	req := &pb.HeartbeatRequest{
		AgentId:   agent.ID,
		Timestamp: timestamp,
		Signature: auth.GenerateSignature(agent.ID, timestamp, agent.Token),
	}
	resp, err := svc.Heartbeat(context.Background(), req)
	if err != nil {
		t.Fatalf("Heartbeat() returned error: %v", err)
	}
	if !resp.Success {
		t.Errorf("Heartbeat() with nil metrics failed: %s", resp.Message)
	}
}

func TestHeartbeatServiceValidation(t *testing.T) {
	if _, err := NewHeartbeatService(nil, nil, &config.Config{}).Heartbeat(context.Background(), &pb.HeartbeatRequest{}); err == nil {
		t.Error("nil database should return an error")
	}
	if _, err := NewHeartbeatService(setupTestDB(t), nil, nil).Heartbeat(context.Background(), &pb.HeartbeatRequest{}); err == nil {
		t.Error("nil config should return an error")
	}
}
