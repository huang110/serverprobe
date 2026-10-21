package handler

import (
	"net/http"
	"testing"
	"time"

	"github.com/huang110/serverprobe/server/internal/model"
)

func TestCreateAlertRule(t *testing.T) {
	api := setupTestAPI(t)
	body := []byte(`{
		"name": "CPU High",
		"metric": "cpu_usage",
		"condition": "gt",
		"threshold": 80.0,
		"duration": 300
	}`)

	recorder := performRequest(t, api.router, http.MethodPost, "/api/alert/rules", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeResponse(t, recorder)
	if !response["success"].(bool) {
		t.Error("success should be true")
	}
	data := response["data"].(map[string]interface{})
	if data["id"].(float64) == 0 {
		t.Error("rule id should be set")
	}
	if data["severity"] != "warning" {
		t.Errorf("severity = %v, want warning", data["severity"])
	}
	if data["duration"].(float64) != 300 {
		t.Errorf("duration = %v, want 300", data["duration"])
	}
	if data["enabled"] != true {
		t.Error("rule should be enabled")
	}

	var rule model.AlertRule
	if err := api.db.First(&rule).Error; err != nil {
		t.Fatalf("failed to query rule: %v", err)
	}
	if rule.Name != "CPU High" || !rule.Enabled {
		t.Errorf("unexpected stored rule: %+v", rule)
	}

	invalid := []byte(`{"name":"bad","metric":"unknown","condition":"gt","threshold":1}`)
	recorder = performRequest(t, api.router, http.MethodPost, "/api/alert/rules", invalid)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid metric status = %d, want 400", recorder.Code)
	}

	missing := []byte(`{"name":"","metric":"cpu_usage","condition":"gt","threshold":1}`)
	recorder = performRequest(t, api.router, http.MethodPost, "/api/alert/rules", missing)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("missing name status = %d, want 400", recorder.Code)
	}
}

func TestListAlertRules(t *testing.T) {
	api := setupTestAPI(t)

	rules := []model.AlertRule{
		{Name: "global", AgentID: "", Metric: "cpu_usage", Condition: "gt", Threshold: 80, Severity: "warning", Enabled: true},
		{Name: "agent rule", AgentID: "agent-1", Metric: "memory_percent", Condition: "gte", Threshold: 90, Severity: "critical", Enabled: true},
		{Name: "disabled", AgentID: "agent-1", Metric: "load1", Condition: "gt", Threshold: 5, Severity: "info", Enabled: false},
	}
	for i := range rules {
		if err := api.db.Create(&rules[i]).Error; err != nil {
			t.Fatalf("failed to create rule: %v", err)
		}
	}
	if err := api.db.Model(&model.AlertRule{}).
		Where("name = ?", "disabled").
		Update("enabled", false).Error; err != nil {
		t.Fatalf("failed to disable rule: %v", err)
	}

	recorder := performRequest(t, api.router, http.MethodGet, "/api/alert/rules?agent_id=agent-1", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	response := decodeResponse(t, recorder)
	data := response["data"].([]interface{})
	if len(data) != 3 {
		t.Errorf("rule count = %d, want 3", len(data))
	}

	recorder = performRequest(t, api.router, http.MethodGet, "/api/alert/rules?enabled=false", nil)
	response = decodeResponse(t, recorder)
	data = response["data"].([]interface{})
	if len(data) != 1 || data[0].(map[string]interface{})["name"] != "disabled" {
		t.Errorf("unexpected disabled rules: %+v", data)
	}

	recorder = performRequest(t, api.router, http.MethodGet, "/api/alert/rules?enabled=bad", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid enabled status = %d, want 400", recorder.Code)
	}
}

func TestListAlertRecords(t *testing.T) {
	api := setupTestAPI(t)
	now := time.Now().Unix()

	records := []model.AlertRecord{
		{RuleID: 1, AgentID: "agent-1", Metric: "cpu_usage", Threshold: 80, ActualValue: 90, Severity: "critical", Status: "firing", FiredAt: now - 60},
		{RuleID: 2, AgentID: "agent-1", Metric: "memory_percent", Threshold: 90, ActualValue: 95, Severity: "warning", Status: "resolved", FiredAt: now - 120},
		{RuleID: 3, AgentID: "agent-2", Metric: "load1", Threshold: 5, ActualValue: 6, Severity: "warning", Status: "firing", FiredAt: now - 180},
	}
	for i := range records {
		if err := api.db.Create(&records[i]).Error; err != nil {
			t.Fatalf("failed to create record: %v", err)
		}
	}

	recorder := performRequest(t, api.router, http.MethodGet,
		"/api/alert/records?agent_id=agent-1&status=firing", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	response := decodeResponse(t, recorder)
	data := response["data"].([]interface{})
	if len(data) != 1 {
		t.Errorf("record count = %d, want 1", len(data))
	}

	recorder = performRequest(t, api.router, http.MethodGet,
		"/api/alert/records?severity=warning", nil)
	response = decodeResponse(t, recorder)
	data = response["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("warning record count = %d, want 2", len(data))
	}

	recorder = performRequest(t, api.router, http.MethodGet,
		"/api/alert/records?status=bad", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid status code = %d, want 400", recorder.Code)
	}
}
