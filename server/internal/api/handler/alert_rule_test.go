package handler

import (
	"net/http"
	"strconv"
	"testing"

	"github.com/huang110/serverprobe/server/internal/model"
)

func TestUpdateAlertRule(t *testing.T) {
	api := setupTestAPI(t)

	createBody := []byte(`{
		"name": "CPU High",
		"agent_id": "agent-1",
		"metric": "cpu_usage",
		"condition": "gt",
		"threshold": 80,
		"duration": 60,
		"severity": "warning"
	}`)
	recorder := performRequest(t, api.router, http.MethodPost, "/api/alert/rules", createBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("create status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	createResponse := decodeResponse(t, recorder)
	ruleID := uint64(createResponse["data"].(map[string]interface{})["id"].(float64))

	updateBody := []byte(`{
		"name": "CPU Critical",
		"threshold": 90,
		"severity": "critical",
		"enabled": false
	}`)
	recorder = performRequest(t, api.router, http.MethodPut,
		"/api/alert/rules/"+uintToString(ruleID), updateBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("update status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	updateResponse := decodeResponse(t, recorder)
	data := updateResponse["data"].(map[string]interface{})
	if data["name"] != "CPU Critical" || data["threshold"].(float64) != 90 ||
		data["severity"] != "critical" || data["enabled"] != false {
		t.Errorf("unexpected updated rule: %+v", data)
	}
	if data["agent_id"] != "agent-1" || data["metric"] != "cpu_usage" || data["condition"] != "gt" {
		t.Errorf("partial update changed unrelated fields: %+v", data)
	}

	var rule model.AlertRule
	if err := api.db.First(&rule, ruleID).Error; err != nil {
		t.Fatalf("failed to query updated rule: %v", err)
	}
	if rule.Name != "CPU Critical" || rule.Threshold != 90 || rule.Enabled {
		t.Errorf("unexpected stored rule: %+v", rule)
	}

	invalidBody := []byte(`{"metric":"unknown"}`)
	recorder = performRequest(t, api.router, http.MethodPut,
		"/api/alert/rules/"+uintToString(ruleID), invalidBody)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid update status = %d, want 400", recorder.Code)
	}

	recorder = performRequest(t, api.router, http.MethodPut, "/api/alert/rules/99999", updateBody)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("missing rule update status = %d, want 404", recorder.Code)
	}
}

func TestDeleteAlertRule(t *testing.T) {
	api := setupTestAPI(t)
	rule := model.AlertRule{
		Name:      "delete me",
		Metric:    "cpu_usage",
		Condition: "gt",
		Threshold: 80,
		Duration:  60,
		Severity:  "warning",
		Enabled:   true,
	}
	if err := api.db.Create(&rule).Error; err != nil {
		t.Fatalf("failed to create rule: %v", err)
	}

	recorder := performRequest(t, api.router, http.MethodDelete,
		"/api/alert/rules/"+uintToString(rule.ID), nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("delete status = %d, body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeResponse(t, recorder)
	if !response["success"].(bool) || response["message"] != "rule deleted" {
		t.Errorf("unexpected delete response: %+v", response)
	}

	var count int64
	api.db.Model(&model.AlertRule{}).Where("id = ?", rule.ID).Count(&count)
	if count != 0 {
		t.Errorf("rule count = %d, want 0", count)
	}

	recorder = performRequest(t, api.router, http.MethodDelete,
		"/api/alert/rules/"+uintToString(rule.ID), nil)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("second delete status = %d, want 404", recorder.Code)
	}
}

func uintToString(value uint64) string {
	return strconv.FormatUint(value, 10)
}
