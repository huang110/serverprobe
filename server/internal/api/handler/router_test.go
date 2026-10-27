package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/model"
)

type testAPI struct {
	db     *gorm.DB
	redis  *cache.RedisClient
	router *gin.Engine
}

// setupTestAPI 创建内存数据库、miniredis 和测试路由
func setupTestAPI(t *testing.T) *testAPI {
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

	gin.SetMode(gin.TestMode)
	router := gin.New()

	agentHandler := NewAgentHandler(db, redisClient)
	metricsHandler := NewMetricsHandler(db, redisClient)
	alertHandler := NewAlertHandler(db)

	router.GET("/api/agents", agentHandler.ListAgents)
	router.GET("/api/agents/:id", agentHandler.GetAgent)
	router.PUT("/api/agents/:id", agentHandler.UpdateAgent)
	router.DELETE("/api/agents/:id", agentHandler.DeleteAgent)

	router.GET("/api/agents/:id/metrics/realtime", metricsHandler.GetRealtimeMetrics)
	router.GET("/api/agents/:id/metrics/history", metricsHandler.GetHistoryMetrics)

	router.POST("/api/alert/rules", alertHandler.CreateRule)
	router.GET("/api/alert/rules", alertHandler.ListRules)
	router.PUT("/api/alert/rules/:id", alertHandler.UpdateRule)
	router.DELETE("/api/alert/rules/:id", alertHandler.DeleteRule)
	router.GET("/api/alert/records", alertHandler.ListRecords)

	return &testAPI{db: db, redis: redisClient, router: router}
}

// performRequest 执行 HTTP 请求
func performRequest(t *testing.T, router *gin.Engine, method, path string, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}

	request, err := http.NewRequest(method, path, reader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)
	return recorder
}

// decodeResponse 解析 JSON 响应
func decodeResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()

	var response map[string]interface{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode response %q: %v", recorder.Body.String(), err)
	}
	return response
}

// createAgent 创建测试 Agent
func createAgent(t *testing.T, db *gorm.DB, id, hostname, status string) *model.Agent {
	t.Helper()

	agent := &model.Agent{
		ID:             id,
		Hostname:       hostname,
		IP:             "10.0.0.1",
		Token:          "secret-token",
		TokenExpiresAt: 1700000000,
		Status:         status,
	}
	if err := db.Create(agent).Error; err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	return agent
}
