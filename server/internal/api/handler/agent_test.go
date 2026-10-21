package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/huang110/serverprobe/server/internal/model"
)

func TestListAgents(t *testing.T) {
	api := setupTestAPI(t)

	createAgent(t, api.db, "agent-1", "web-1", "online")
	createAgent(t, api.db, "agent-2", "web-2", "offline")

	recorder := performRequest(t, api.router, http.MethodGet, "/api/agents", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeResponse(t, recorder)
	if !response["success"].(bool) {
		t.Error("success should be true")
	}
	data := response["data"].(map[string]interface{})
	if data["total"].(float64) != 2 {
		t.Errorf("total = %v, want 2", data["total"])
	}
	if len(data["agents"].([]interface{})) != 2 {
		t.Errorf("agents length = %d, want 2", len(data["agents"].([]interface{})))
	}
	if strings.Contains(recorder.Body.String(), "secret-token") {
		t.Error("agent token should not be serialized")
	}

	// 状态过滤
	recorder = performRequest(t, api.router, http.MethodGet, "/api/agents?status=online", nil)
	response = decodeResponse(t, recorder)
	data = response["data"].(map[string]interface{})
	if data["total"].(float64) != 1 {
		t.Errorf("online total = %v, want 1", data["total"])
	}

	// 关键词搜索
	recorder = performRequest(t, api.router, http.MethodGet, "/api/agents?keyword=web-2", nil)
	response = decodeResponse(t, recorder)
	data = response["data"].(map[string]interface{})
	if data["total"].(float64) != 1 {
		t.Errorf("keyword total = %v, want 1", data["total"])
	}

	// 分页
	recorder = performRequest(t, api.router, http.MethodGet, "/api/agents?page=1&page_size=1", nil)
	response = decodeResponse(t, recorder)
	data = response["data"].(map[string]interface{})
	if len(data["agents"].([]interface{})) != 1 || data["page_size"].(float64) != 1 {
		t.Errorf("unexpected pagination result: %+v", data)
	}

	// 非法状态
	recorder = performRequest(t, api.router, http.MethodGet, "/api/agents?status=unknown", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid status code = %d, want 400", recorder.Code)
	}
}

func TestGetAgent(t *testing.T) {
	api := setupTestAPI(t)
	agent := createAgent(t, api.db, "agent-1", "web-1", "online")
	if err := api.db.Create(&model.AgentLabel{
		AgentID: agent.ID,
		Key:     "env",
		Value:   "prod",
	}).Error; err != nil {
		t.Fatalf("failed to create label: %v", err)
	}
	if err := api.redis.SaveRealtimeMetrics(agent.ID, map[string]interface{}{
		"cpu_usage": 25.5,
	}); err != nil {
		t.Fatalf("failed to save realtime metrics: %v", err)
	}

	recorder := performRequest(t, api.router, http.MethodGet, "/api/agents/agent-1", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	if data["agent"].(map[string]interface{})["id"] != "agent-1" {
		t.Errorf("unexpected agent: %+v", data["agent"])
	}
	if data["labels"].(map[string]interface{})["env"] != "prod" {
		t.Errorf("unexpected labels: %+v", data["labels"])
	}
	if data["realtime"].(map[string]interface{})["cpu_usage"] != "25.5" {
		t.Errorf("unexpected realtime: %+v", data["realtime"])
	}

	recorder = performRequest(t, api.router, http.MethodGet, "/api/agents/missing", nil)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("missing agent status = %d, want 404", recorder.Code)
	}
}

func TestUpdateAgent(t *testing.T) {
	api := setupTestAPI(t)
	agent := createAgent(t, api.db, "agent-1", "web-1", "online")
	if err := api.db.Create(&model.AgentLabel{
		AgentID: agent.ID,
		Key:     "env",
		Value:   "prod",
	}).Error; err != nil {
		t.Fatalf("failed to create label: %v", err)
	}

	body := []byte(`{"labels":{"env":"staging","owner":"devops"}}`)
	recorder := performRequest(t, api.router, http.MethodPut, "/api/agents/agent-1", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var labels []model.AgentLabel
	if err := api.db.Where("agent_id = ?", agent.ID).Find(&labels).Error; err != nil {
		t.Fatalf("failed to query labels: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("label count = %d, want 2", len(labels))
	}
	labelMap := make(map[string]string, len(labels))
	for _, label := range labels {
		labelMap[label.Key] = label.Value
	}
	if labelMap["env"] != "staging" || labelMap["owner"] != "devops" {
		t.Errorf("unexpected labels: %+v", labelMap)
	}

	recorder = performRequest(t, api.router, http.MethodPut, "/api/agents/agent-1", []byte(`not-json`))
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid body status = %d, want 400", recorder.Code)
	}

	recorder = performRequest(t, api.router, http.MethodPut, "/api/agents/missing", body)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("missing agent status = %d, want 404", recorder.Code)
	}
}

func TestDeleteAgent(t *testing.T) {
	api := setupTestAPI(t)
	agent := createAgent(t, api.db, "agent-1", "web-1", "online")
	if err := api.db.Create(&model.AgentLabel{
		AgentID: agent.ID,
		Key:     "env",
		Value:   "prod",
	}).Error; err != nil {
		t.Fatalf("failed to create label: %v", err)
	}
	if err := api.redis.SaveRealtimeMetrics(agent.ID, map[string]interface{}{"cpu_usage": 1}); err != nil {
		t.Fatalf("failed to save realtime metrics: %v", err)
	}
	if err := api.redis.UpdateOnlineAgent(agent.ID, time.Now().Unix()); err != nil {
		t.Fatalf("failed to update online agent: %v", err)
	}

	recorder := performRequest(t, api.router, http.MethodDelete, "/api/agents/agent-1", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	var agentCount, labelCount int64
	api.db.Model(&model.Agent{}).Where("id = ?", agent.ID).Count(&agentCount)
	api.db.Model(&model.AgentLabel{}).Where("agent_id = ?", agent.ID).Count(&labelCount)
	if agentCount != 0 || labelCount != 0 {
		t.Errorf("agent count=%d label count=%d, want 0/0", agentCount, labelCount)
	}

	realtime, err := api.redis.GetRealtimeMetrics(agent.ID)
	if err != nil {
		t.Fatalf("failed to read realtime metrics: %v", err)
	}
	if len(realtime) != 0 {
		t.Errorf("realtime cache should be empty, got %+v", realtime)
	}
	online, err := api.redis.GetOnlineAgents(300)
	if err != nil {
		t.Fatalf("failed to get online agents: %v", err)
	}
	if len(online) != 0 {
		t.Errorf("online set should be empty, got %v", online)
	}

	recorder = performRequest(t, api.router, http.MethodDelete, "/api/agents/agent-1", nil)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", recorder.Code)
	}
}
