package grpc

import (
	"context"
	"testing"
	"time"

	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
	"github.com/huang110/serverprobe/server/internal/pkg/auth"
	pb "github.com/huang110/serverprobe/server/proto"
)

func TestGetConfigSuccess(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	cfg := &config.Config{
		HeartbeatInterval:   60,
		EnableIPQuality:     true,
		TokenExpirationDays: 7,
	}
	svc := NewConfigService(db, redisClient, cfg)

	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())
	if err := db.Create(&model.AgentLabel{AgentID: agent.ID, Key: "env", Value: "prod"}).Error; err != nil {
		t.Fatalf("failed to create label: %v", err)
	}

	timestamp := time.Now().Unix()
	req := &pb.GetConfigRequest{
		AgentId:   agent.ID,
		Timestamp: timestamp,
		Signature: auth.GenerateSignature(agent.ID, timestamp, agent.Token),
	}

	resp, err := svc.GetConfig(context.Background(), req)
	if err != nil {
		t.Fatalf("GetConfig() returned error: %v", err)
	}
	if !resp.Success {
		t.Fatal("GetConfig() failed")
	}
	if resp.HeartbeatInterval != 60 {
		t.Errorf("HeartbeatInterval = %d, want 60", resp.HeartbeatInterval)
	}
	if resp.Labels["env"] != "prod" {
		t.Errorf("Labels[env] = %q, want %q", resp.Labels["env"], "prod")
	}
	if !resp.EnableIpQuality {
		t.Error("EnableIpQuality should be true")
	}
}

func TestGetConfigInvalidSignature(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewConfigService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())

	resp, err := svc.GetConfig(context.Background(), &pb.GetConfigRequest{
		AgentId:   agent.ID,
		Timestamp: time.Now().Unix(),
		Signature: "invalid",
	})
	if err != nil {
		t.Fatalf("GetConfig() returned error: %v", err)
	}
	if resp.Success {
		t.Error("GetConfig() should reject invalid signature")
	}
}

func TestGetConfigExpiredTimestamp(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewConfigService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "test-token", time.Now().Add(48*time.Hour).Unix())

	timestamp := time.Now().Add(-10 * time.Minute).Unix()
	resp, err := svc.GetConfig(context.Background(), &pb.GetConfigRequest{
		AgentId:   agent.ID,
		Timestamp: timestamp,
		Signature: auth.GenerateSignature(agent.ID, timestamp, agent.Token),
	})
	if err != nil {
		t.Fatalf("GetConfig() returned error: %v", err)
	}
	if resp.Success {
		t.Error("GetConfig() should reject expired timestamp")
	}
}

func TestGetConfigAgentNotFound(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewConfigService(db, redisClient, &config.Config{HeartbeatInterval: 30})

	resp, err := svc.GetConfig(context.Background(), &pb.GetConfigRequest{
		AgentId:   "missing",
		Timestamp: time.Now().Unix(),
		Signature: "invalid",
	})
	if err != nil {
		t.Fatalf("GetConfig() returned error: %v", err)
	}
	if resp.Success {
		t.Error("GetConfig() should reject unknown agent")
	}
}

func TestGetConfigNilRequest(t *testing.T) {
	svc := NewConfigService(nil, nil, nil)
	resp, err := svc.GetConfig(context.Background(), nil)
	if err != nil {
		t.Fatalf("GetConfig() returned error: %v", err)
	}
	if resp.Success {
		t.Error("nil request should fail")
	}
}
