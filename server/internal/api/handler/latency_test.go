package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/huang110/serverprobe/server/internal/model"
)

func TestGetLatencyHistory(t *testing.T) {
	api := setupTestAPI(t)
	now := time.Now().Unix()

	rows := []model.MetricsLatency{
		{AgentID: "agent-1", TargetName: "电信", Target: "202.102.192.68:53", Timestamp: now - 30, LatencyMs: 42.5, JitterMs: 2.1},
		{AgentID: "agent-1", TargetName: "电信", Target: "202.102.192.68:53", Timestamp: now - 15, LatencyMs: 43.1, JitterMs: 1.8},
		{AgentID: "agent-1", TargetName: "联通", Target: "210.22.70.3:53", Timestamp: now - 20, LatencyMs: 48.2, JitterMs: 3.4, PacketLoss: 33.33},
		{AgentID: "agent-2", TargetName: "电信", Target: "202.102.192.68:53", Timestamp: now - 10, LatencyMs: 99.9, JitterMs: 9.9},
	}
	if err := api.db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to create latency metrics: %v", err)
	}

	path := fmt.Sprintf("/api/agents/agent-1/metrics/latency?start=%d&end=%d", now-60, now)
	recorder := performRequest(t, api.router, http.MethodGet, path, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	series := data["series"].([]interface{})
	if len(series) != 2 {
		t.Fatalf("series count = %d, want 2: %+v", len(series), series)
	}

	first := series[0].(map[string]interface{})
	if first["target_name"] != "电信" || first["target"] != "202.102.192.68:53" {
		t.Errorf("unexpected first series: %+v", first)
	}
	points := first["points"].([]interface{})
	if len(points) != 2 {
		t.Fatalf("first series point count = %d, want 2", len(points))
	}
	firstPoint := points[0].(map[string]interface{})
	secondPoint := points[1].(map[string]interface{})
	if firstPoint["timestamp"].(float64) >= secondPoint["timestamp"].(float64) {
		t.Error("points should be ordered by timestamp ASC")
	}
	if firstPoint["latency_ms"].(float64) != 42.5 || firstPoint["jitter_ms"].(float64) != 2.1 {
		t.Errorf("unexpected point: %+v", firstPoint)
	}

	second := series[1].(map[string]interface{})
	if second["target_name"] != "联通" {
		t.Errorf("series should be sorted by target_name: %+v", series)
	}
	secondPoints := second["points"].([]interface{})
	if len(secondPoints) != 1 {
		t.Fatalf("second series point count = %d, want 1", len(secondPoints))
	}
	if secondPoints[0].(map[string]interface{})["packet_loss"].(float64) != 33.33 {
		t.Errorf("unexpected packet loss: %+v", secondPoints[0])
	}
}

func TestGetLatencyHistoryEmpty(t *testing.T) {
	api := setupTestAPI(t)

	recorder := performRequest(t, api.router, http.MethodGet,
		"/api/agents/missing/metrics/latency", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	series, ok := data["series"].([]interface{})
	if !ok || len(series) != 0 {
		t.Errorf("series = %#v, want empty array", data["series"])
	}
}

func TestGetLatencyHistoryValidation(t *testing.T) {
	api := setupTestAPI(t)

	recorder := performRequest(t, api.router, http.MethodGet,
		"/api/agents/agent-1/metrics/latency?start=100&end=100", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid range status = %d, want 400", recorder.Code)
	}
}

func TestGetLatencyHistoryPointLimit(t *testing.T) {
	api := setupTestAPI(t)
	now := time.Now().Unix()

	rows := make([]model.MetricsLatency, 0, 2005)
	for index := 0; index < 2005; index++ {
		rows = append(rows, model.MetricsLatency{
			AgentID:    "agent-1",
			TargetName: "大量",
			Target:     "223.5.5.5:443",
			Timestamp:  now - int64(2005-index),
			LatencyMs:  float64(index),
		})
	}
	if err := api.db.Create(&rows).Error; err != nil {
		t.Fatalf("failed to create latency metrics: %v", err)
	}

	path := "/api/agents/agent-1/metrics/latency?start=" +
		strconv.FormatInt(now-3600, 10) + "&end=" + strconv.FormatInt(now, 10)
	recorder := performRequest(t, api.router, http.MethodGet, path, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	series := data["series"].([]interface{})
	if len(series) != 1 {
		t.Fatalf("series count = %d, want 1", len(series))
	}
	points := series[0].(map[string]interface{})["points"].([]interface{})
	if len(points) != 2000 {
		t.Errorf("point count = %d, want 2000", len(points))
	}
}
