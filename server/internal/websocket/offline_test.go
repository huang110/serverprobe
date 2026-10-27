package websocket

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/model"
)

func TestCheckOfflineAgents(t *testing.T) {
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

	redisServer := miniredis.RunT(t)
	redisClient, err := cache.NewRedisClient(redisServer.Addr(), "", 0)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := redisClient.Close(); closeErr != nil {
			t.Errorf("failed to close redis client: %v", closeErr)
		}
	})

	now := time.Now().Unix()
	agents := []model.Agent{
		{ID: "old-agent", Hostname: "old", Token: "t", Status: "online", LastHeartbeatAt: now - 120},
		{ID: "recent-agent", Hostname: "recent", Token: "t", Status: "online", LastHeartbeatAt: now - 5},
		{ID: "offline-agent", Hostname: "offline", Token: "t", Status: "offline", LastHeartbeatAt: now - 300},
	}
	for index := range agents {
		if err := db.Create(&agents[index]).Error; err != nil {
			t.Fatalf("failed to create agent: %v", err)
		}
	}
	if err := redisClient.SaveRealtimeMetrics("old-agent", map[string]interface{}{
		"cpu_usage": 1,
		"status":    "online",
	}); err != nil {
		t.Fatalf("failed to save realtime metrics: %v", err)
	}

	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	client := &Client{send: make(chan []byte, 4)}
	hub.Register(client)
	waitForClientCount(t, hub, 1)

	broadcaster := NewBroadcaster(hub, redisClient, db)
	broadcaster.checkOfflineAgents(context.Background())

	var oldAgent model.Agent
	if err := db.First(&oldAgent, "id = ?", "old-agent").Error; err != nil {
		t.Fatalf("failed to query old agent: %v", err)
	}
	if oldAgent.Status != "offline" {
		t.Errorf("old agent status = %q, want offline", oldAgent.Status)
	}

	var recentAgent model.Agent
	if err := db.First(&recentAgent, "id = ?", "recent-agent").Error; err != nil {
		t.Fatalf("failed to query recent agent: %v", err)
	}
	if recentAgent.Status != "online" {
		t.Errorf("recent agent status = %q, want online", recentAgent.Status)
	}

	realtime, err := redisClient.GetRealtimeMetrics("old-agent")
	if err != nil {
		t.Fatalf("failed to get realtime metrics: %v", err)
	}
	if realtime["status"] != "offline" {
		t.Errorf("redis status = %q, want offline", realtime["status"])
	}

	select {
	case payload := <-client.send:
		var message map[string]interface{}
		if err := json.Unmarshal(payload, &message); err != nil {
			t.Fatalf("failed to unmarshal websocket message: %v", err)
		}
		if message["type"] != "agent_status" {
			t.Errorf("message type = %v, want agent_status", message["type"])
		}
		data := message["data"].(map[string]interface{})
		if data["agent_id"] != "old-agent" || data["status"] != "offline" {
			t.Errorf("unexpected offline message: %+v", data)
		}
	case <-time.After(time.Second):
		t.Fatal("offline status was not broadcast")
	}
}
