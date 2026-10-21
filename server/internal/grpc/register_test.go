package grpc

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
	pb "github.com/huang110/serverprobe/server/proto"
)

// setupTestDB 创建内存 SQLite 测试数据库
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	t.Cleanup(func() {
		if closeErr := sqlDB.Close(); closeErr != nil {
			t.Errorf("failed to close database: %v", closeErr)
		}
	})

	if err := model.AutoMigrate(db); err != nil {
		t.Fatalf("failed to migrate database: %v", err)
	}

	return db
}

func newTestConfig() *config.Config {
	return &config.Config{
		RegisterSecret:      "test-secret",
		HeartbeatInterval:   30,
		TokenExpirationDays: 7,
	}
}

func TestRegisterNewAgent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRegisterService(db, newTestConfig())

	req := &pb.RegisterRequest{
		RegisterSecret: "test-secret",
		Hostname:       "node-1",
		Ip:             "203.0.113.10",
		InternalIp:     "10.0.0.5",
		Os:             "linux",
		OsVersion:      "Ubuntu 22.04",
		KernelVersion:  "5.15.0",
		CpuCores:       8,
		MemoryTotal:    16384,
		DiskTotal:      512000,
		Arch:           "amd64",
		AgentVersion:   "1.0.0",
		Labels:         map[string]string{"env": "prod", "role": "api"},
		LiteMode:       true,
	}

	before := time.Now().Unix()
	resp, err := svc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}
	if !resp.Success {
		t.Fatalf("Register() success=false, message=%s", resp.Message)
	}
	if resp.AgentId == "" || resp.Token == "" {
		t.Fatal("AgentId or Token is empty")
	}
	if _, err := uuid.Parse(resp.AgentId); err != nil {
		t.Errorf("AgentId is not a valid UUID: %v", err)
	}
	if len(resp.Token) != tokenLength*2 {
		t.Errorf("Token length = %d, want %d", len(resp.Token), tokenLength*2)
	}
	if _, err := hex.DecodeString(resp.Token); err != nil {
		t.Errorf("Token is not valid hex: %v", err)
	}
	if resp.TokenExpiresAt <= before {
		t.Errorf("TokenExpiresAt = %d, want > %d", resp.TokenExpiresAt, before)
	}
	if resp.HeartbeatInterval != 30 {
		t.Errorf("HeartbeatInterval = %d, want 30", resp.HeartbeatInterval)
	}

	var agent model.Agent
	if err := db.First(&agent, "id = ?", resp.AgentId).Error; err != nil {
		t.Fatalf("Agent not found in database: %v", err)
	}
	if agent.Hostname != "node-1" || agent.IP != "203.0.113.10" || agent.InternalIP != "10.0.0.5" {
		t.Errorf("unexpected agent fields: %+v", agent)
	}
	if agent.CPUCores != 8 || agent.MemoryTotal != 16384 || agent.DiskTotal != 512000 {
		t.Errorf("unexpected system fields: %+v", agent)
	}
	if agent.Status != agentStatusOnline || !agent.LiteMode {
		t.Errorf("unexpected status/lite mode: status=%s lite=%v", agent.Status, agent.LiteMode)
	}
	if agent.Token != resp.Token {
		t.Errorf("stored token does not match response")
	}

	var labels []model.AgentLabel
	if err := db.Where("agent_id = ?", resp.AgentId).Find(&labels).Error; err != nil {
		t.Fatalf("failed to query labels: %v", err)
	}
	if len(labels) != 2 {
		t.Errorf("label count = %d, want 2", len(labels))
	}
	labelMap := make(map[string]string, len(labels))
	for _, label := range labels {
		labelMap[label.Key] = label.Value
	}
	if labelMap["env"] != "prod" || labelMap["role"] != "api" {
		t.Errorf("unexpected labels: %+v", labelMap)
	}
}

func TestRegisterInvalidSecret(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRegisterService(db, newTestConfig())

	resp, err := svc.Register(context.Background(), &pb.RegisterRequest{
		RegisterSecret: "wrong-secret",
		Hostname:       "node-1",
	})
	if err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}
	if resp.Success {
		t.Error("Register() should reject invalid secret")
	}
	if resp.Message != "invalid register secret" {
		t.Errorf("Message = %q, want %q", resp.Message, "invalid register secret")
	}

	var count int64
	db.Model(&model.Agent{}).Count(&count)
	if count != 0 {
		t.Errorf("agent count = %d, want 0", count)
	}
}

func TestRegisterMissingHostname(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRegisterService(db, newTestConfig())

	resp, err := svc.Register(context.Background(), &pb.RegisterRequest{
		RegisterSecret: "test-secret",
		Hostname:       "   ",
	})
	if err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}
	if resp.Success {
		t.Error("Register() should reject empty hostname")
	}
}

func TestRegisterExistingAgent(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRegisterService(db, newTestConfig())

	firstReq := &pb.RegisterRequest{
		RegisterSecret: "test-secret",
		Hostname:       "node-1",
		Ip:             "203.0.113.10",
		Labels:         map[string]string{"env": "prod"},
	}
	first, err := svc.Register(context.Background(), firstReq)
	if err != nil {
		t.Fatalf("first Register() returned error: %v", err)
	}

	secondReq := &pb.RegisterRequest{
		RegisterSecret: "test-secret",
		Hostname:       "node-1",
		Ip:             "203.0.113.20",
		Labels:         map[string]string{"env": "staging", "role": "api"},
	}
	second, err := svc.Register(context.Background(), secondReq)
	if err != nil {
		t.Fatalf("second Register() returned error: %v", err)
	}

	if second.AgentId != first.AgentId {
		t.Errorf("AgentId changed: %q -> %q", first.AgentId, second.AgentId)
	}
	if second.Token != first.Token {
		t.Error("Token changed for non-expired agent")
	}
	if second.Message != "agent already registered" {
		t.Errorf("Message = %q, want %q", second.Message, "agent already registered")
	}

	var agent model.Agent
	db.First(&agent, "id = ?", first.AgentId)
	if agent.IP != "203.0.113.20" {
		t.Errorf("IP = %q, want %q", agent.IP, "203.0.113.20")
	}

	var labels []model.AgentLabel
	db.Where("agent_id = ?", first.AgentId).Find(&labels)
	if len(labels) != 2 {
		t.Fatalf("label count = %d, want 2", len(labels))
	}
	for _, label := range labels {
		if label.Key == "env" && label.Value != "staging" {
			t.Errorf("old label was not replaced: %+v", label)
		}
	}
}

func TestRegisterExpiredToken(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRegisterService(db, newTestConfig())

	req := &pb.RegisterRequest{
		RegisterSecret: "test-secret",
		Hostname:       "node-1",
	}
	first, err := svc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("first Register() returned error: %v", err)
	}

	if err := db.Model(&model.Agent{}).
		Where("id = ?", first.AgentId).
		Update("token_expires_at", time.Now().Add(-time.Hour).Unix()).Error; err != nil {
		t.Fatalf("failed to expire token: %v", err)
	}

	second, err := svc.Register(context.Background(), req)
	if err != nil {
		t.Fatalf("second Register() returned error: %v", err)
	}
	if second.AgentId != first.AgentId {
		t.Errorf("AgentId changed on token renewal: %q -> %q", first.AgentId, second.AgentId)
	}
	if second.Token == first.Token {
		t.Error("Token should be renewed after expiration")
	}
	if second.TokenExpiresAt <= time.Now().Unix() {
		t.Error("renewed TokenExpiresAt should be in the future")
	}
	if second.Message != "token renewed" {
		t.Errorf("Message = %q, want %q", second.Message, "token renewed")
	}
}

func TestRegisterDefaultHeartbeatInterval(t *testing.T) {
	db := setupTestDB(t)
	svc := NewRegisterService(db, &config.Config{RegisterSecret: "test-secret"})

	resp, err := svc.Register(context.Background(), &pb.RegisterRequest{
		RegisterSecret: "test-secret",
		Hostname:       "node-1",
	})
	if err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}
	if resp.HeartbeatInterval != config.DefaultHeartbeatInterval {
		t.Errorf("HeartbeatInterval = %d, want %d", resp.HeartbeatInterval, config.DefaultHeartbeatInterval)
	}
}

func TestGenerateToken(t *testing.T) {
	first, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() failed: %v", err)
	}
	second, err := generateToken()
	if err != nil {
		t.Fatalf("generateToken() failed: %v", err)
	}
	if len(first) != tokenLength*2 {
		t.Errorf("token length = %d, want %d", len(first), tokenLength*2)
	}
	if first == second {
		t.Error("generated tokens should be unique")
	}
	if _, err := hex.DecodeString(first); err != nil {
		t.Errorf("token is not valid hex: %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	if _, err := NewRegisterService(nil, newTestConfig()).Register(context.Background(), &pb.RegisterRequest{}); err == nil {
		t.Error("nil database should return an error")
	}
	if _, err := NewRegisterService(setupTestDB(t), nil).Register(context.Background(), &pb.RegisterRequest{}); err == nil {
		t.Error("nil config should return an error")
	}
	if _, err := NewRegisterService(setupTestDB(t), newTestConfig()).Register(context.Background(), nil); err == nil {
		t.Error("nil request should return an error")
	}
}
