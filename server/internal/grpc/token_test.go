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

func TestRotateTokenSuccess(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	cfg := &config.Config{HeartbeatInterval: 30, TokenExpirationDays: 7}
	svc := NewTokenService(db, redisClient, cfg)

	agent := createTestAgent(t, db, "agent-1", "old-token", time.Now().Add(48*time.Hour).Unix())
	timestamp := time.Now().Unix()
	req := &pb.RotateTokenRequest{
		AgentId:   agent.ID,
		Timestamp: timestamp,
		Signature: auth.GenerateSignature(agent.ID, timestamp, agent.Token),
	}

	resp, err := svc.RotateToken(context.Background(), req)
	if err != nil {
		t.Fatalf("RotateToken() returned error: %v", err)
	}
	if !resp.Success {
		t.Fatal("RotateToken() failed")
	}
	if resp.NewToken == "" || resp.NewToken == "old-token" {
		t.Error("NewToken should be different from old token")
	}
	if resp.ExpiresAt <= time.Now().Unix() {
		t.Error("ExpiresAt should be in the future")
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
	if updated.Token != resp.NewToken || updated.TokenExpiresAt != resp.ExpiresAt {
		t.Errorf("agent token not updated: %+v", updated)
	}

	// 新 Token 可用于签名验证
	configSvc := NewConfigService(db, redisClient, cfg)
	now := time.Now().Unix()
	configResp, err := configSvc.GetConfig(context.Background(), &pb.GetConfigRequest{
		AgentId:   agent.ID,
		Timestamp: now,
		Signature: auth.GenerateSignature(agent.ID, now, resp.NewToken),
	})
	if err != nil {
		t.Fatalf("GetConfig() with new token returned error: %v", err)
	}
	if !configResp.Success {
		t.Error("new token should be accepted")
	}

	// 旧 Token 已被吊销
	oldResp, err := configSvc.GetConfig(context.Background(), &pb.GetConfigRequest{
		AgentId:   agent.ID,
		Timestamp: now,
		Signature: auth.GenerateSignature(agent.ID, now, "old-token"),
	})
	if err != nil {
		t.Fatalf("GetConfig() with old token returned error: %v", err)
	}
	if oldResp.Success {
		t.Error("old token should be rejected after rotation")
	}
}

func TestRotateTokenInvalidSignature(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewTokenService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "old-token", time.Now().Add(48*time.Hour).Unix())

	resp, err := svc.RotateToken(context.Background(), &pb.RotateTokenRequest{
		AgentId:   agent.ID,
		Timestamp: time.Now().Unix(),
		Signature: "invalid",
	})
	if err != nil {
		t.Fatalf("RotateToken() returned error: %v", err)
	}
	if resp.Success {
		t.Error("RotateToken() should reject invalid signature")
	}
}

func TestRotateTokenExpiredTimestamp(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewTokenService(db, redisClient, &config.Config{HeartbeatInterval: 30})
	agent := createTestAgent(t, db, "agent-1", "old-token", time.Now().Add(48*time.Hour).Unix())

	timestamp := time.Now().Add(-10 * time.Minute).Unix()
	resp, err := svc.RotateToken(context.Background(), &pb.RotateTokenRequest{
		AgentId:   agent.ID,
		Timestamp: timestamp,
		Signature: auth.GenerateSignature(agent.ID, timestamp, agent.Token),
	})
	if err != nil {
		t.Fatalf("RotateToken() returned error: %v", err)
	}
	if resp.Success {
		t.Error("RotateToken() should reject expired timestamp")
	}
}

func TestRotateTokenNilRequest(t *testing.T) {
	svc := NewTokenService(nil, nil, nil)
	resp, err := svc.RotateToken(context.Background(), nil)
	if err != nil {
		t.Fatalf("RotateToken() returned error: %v", err)
	}
	if resp.Success {
		t.Error("nil request should fail")
	}
}

func TestRotateTokenAgentNotFound(t *testing.T) {
	db := setupTestDB(t)
	redisClient := setupTestRedis(t)
	svc := NewTokenService(db, redisClient, &config.Config{HeartbeatInterval: 30})

	resp, err := svc.RotateToken(context.Background(), &pb.RotateTokenRequest{
		AgentId:   "missing",
		Timestamp: time.Now().Unix(),
		Signature: "invalid",
	})
	if err != nil {
		t.Fatalf("RotateToken() returned error: %v", err)
	}
	if resp.Success {
		t.Error("RotateToken() should reject unknown agent")
	}
}
