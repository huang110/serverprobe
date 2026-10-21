package alert

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/model"
)

func setupAlertTestDB(t *testing.T) *gorm.DB {
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

func setupAlertTestRedis(t *testing.T) *cache.RedisClient {
	t.Helper()

	server := miniredis.RunT(t)
	client, err := cache.NewRedisClient(server.Addr(), "", 0)
	if err != nil {
		t.Fatalf("failed to create redis client: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("failed to close redis client: %v", closeErr)
		}
	})
	return client
}

func TestEngineFiringAndResolved(t *testing.T) {
	db := setupAlertTestDB(t)
	redisClient := setupAlertTestRedis(t)
	cfg := &config.Config{Alert: config.AlertConfig{CheckInterval: 1}}

	agent := &model.Agent{
		ID:             "agent-1",
		Hostname:       "node-1",
		Token:          "token-1",
		TokenExpiresAt: time.Now().Add(48 * time.Hour).Unix(),
		Status:         "online",
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	var mu sync.Mutex
	var webhookPayloads []map[string]interface{}
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			mu.Lock()
			webhookPayloads = append(webhookPayloads, payload)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	rule := &model.AlertRule{
		Name:       "CPU High",
		AgentID:    agent.ID,
		Metric:     "cpu_usage",
		Condition:  "gt",
		Threshold:  80,
		Duration:   0,
		Severity:   "critical",
		Enabled:    true,
		WebhookURL: webhook.URL,
	}
	if err := db.Create(rule).Error; err != nil {
		t.Fatalf("failed to create rule: %v", err)
	}
	// GORM 对 default:60 的字段会在插入时填充默认值，这里显式改为 0 以便立即触发
	if err := db.Model(rule).Update("duration", 0).Error; err != nil {
		t.Fatalf("failed to update rule duration: %v", err)
	}

	if err := redisClient.SaveRealtimeMetrics(agent.ID, map[string]interface{}{"cpu_usage": 95.0}); err != nil {
		t.Fatalf("failed to save realtime metrics: %v", err)
	}

	engine := NewEngine(db, redisClient, cfg)
	engine.checkRules(context.Background())

	var records []model.AlertRecord
	if err := db.Where("agent_id = ?", agent.ID).Find(&records).Error; err != nil {
		t.Fatalf("failed to query alert records: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("alert record count = %d, want 1", len(records))
	}
	if records[0].Status != "firing" || records[0].ActualValue != 95 {
		t.Errorf("unexpected alert record: %+v", records[0])
	}
	if records[0].NotifiedAt == 0 {
		t.Error("notified_at should be set after webhook delivery")
	}

	// 重复检查不应产生新的告警记录
	engine.checkRules(context.Background())
	var count int64
	db.Model(&model.AlertRecord{}).Where("agent_id = ?", agent.ID).Count(&count)
	if count != 1 {
		t.Errorf("alert record count after second check = %d, want 1", count)
	}

	// 指标恢复
	if err := redisClient.SaveRealtimeMetrics(agent.ID, map[string]interface{}{"cpu_usage": 5.0}); err != nil {
		t.Fatalf("failed to save realtime metrics: %v", err)
	}
	engine.checkRules(context.Background())

	var resolved model.AlertRecord
	if err := db.First(&resolved, records[0].ID).Error; err != nil {
		t.Fatalf("failed to query resolved record: %v", err)
	}
	if resolved.Status != "resolved" || resolved.ResolvedAt == 0 {
		t.Errorf("record should be resolved: %+v", resolved)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(webhookPayloads) != 2 {
		t.Fatalf("webhook payload count = %d, want 2", len(webhookPayloads))
	}
	if webhookPayloads[0]["status"] != "firing" || webhookPayloads[1]["status"] != "resolved" {
		t.Errorf("unexpected webhook statuses: %v, %v", webhookPayloads[0]["status"], webhookPayloads[1]["status"])
	}
	if webhookPayloads[0]["agent_hostname"] != "node-1" {
		t.Errorf("webhook hostname = %v, want node-1", webhookPayloads[0]["agent_hostname"])
	}
}

func TestEngineStartStop(t *testing.T) {
	db := setupAlertTestDB(t)
	redisClient := setupAlertTestRedis(t)
	cfg := &config.Config{Alert: config.AlertConfig{CheckInterval: 1}}
	engine := NewEngine(db, redisClient, cfg)

	done := make(chan struct{})
	go func() {
		engine.Start()
		close(done)
	}()

	time.Sleep(20 * time.Millisecond)
	engine.Stop()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("engine did not stop")
	}
}

func TestNotifierSkipsWithoutWebhook(t *testing.T) {
	notifier := NewNotifier(&config.Config{})
	if err := notifier.Notify(context.Background(), &model.AlertRule{}, "agent-1", "node-1", 1, "firing"); err != nil {
		t.Errorf("Notify() without webhook should not fail: %v", err)
	}
}

func TestNotifierWebhookError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	notifier := NewNotifier(&config.Config{})
	err := notifier.Notify(context.Background(), &model.AlertRule{
		ID:         1,
		Name:       "rule",
		Metric:     "cpu_usage",
		Condition:  "gt",
		Threshold:  80,
		WebhookURL: server.URL,
	}, "agent-1", "node-1", 90, "firing")
	if err == nil {
		t.Error("expected webhook error for 500 response")
	}
}
