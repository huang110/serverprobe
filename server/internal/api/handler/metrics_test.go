package handler

import (
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/huang110/serverprobe/server/internal/model"
)

func TestGetRealtimeMetrics(t *testing.T) {
	api := setupTestAPI(t)

	if err := api.redis.SaveRealtimeMetrics("agent-1", map[string]interface{}{
		"cpu_usage":       25.5,
		"cpu_steal":       0.2,
		"cpu_iowait":      0.1,
		"memory_percent":  60.0,
		"swap_total":      1024,
		"swap_used":       128,
		"swap_percent":    12.5,
		"load1":           1.5,
		"net_rx_total":    10737418240,
		"net_tx_total":    5368709120,
		"uptime":          518400,
		"process_count":   128,
		"tcp_established": 42,
		"tcp_time_wait":   5,
		"status":          "online",
	}); err != nil {
		t.Fatalf("failed to save realtime metrics: %v", err)
	}
	if err := api.redis.SaveDiskMetrics("agent-1", map[string]interface{}{
		"/": map[string]interface{}{"usage_percent": 65.0, "io_util": 10.5},
	}); err != nil {
		t.Fatalf("failed to save disk metrics: %v", err)
	}
	if err := api.redis.SaveNetworkMetrics("agent-1", map[string]interface{}{
		"eth0": map[string]interface{}{"rx_bytes": 1024, "tx_bytes": 2048},
	}); err != nil {
		t.Fatalf("failed to save network metrics: %v", err)
	}
	if err := api.redis.SaveLatencyMetrics("agent-1", map[string]interface{}{
		"联通": model.LatencyItem{
			TargetName: "联通", Target: "210.22.70.3:53",
			LatencyMs: 48.2, JitterMs: 3.4, PacketLoss: 0,
		},
		"电信": model.LatencyItem{
			TargetName: "电信", Target: "202.102.192.68:53",
			LatencyMs: 42.5, JitterMs: 2.1, PacketLoss: 33.33,
		},
	}); err != nil {
		t.Fatalf("failed to save latency metrics: %v", err)
	}

	recorder := performRequest(t, api.router, http.MethodGet, "/api/agents/agent-1/metrics/realtime", nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	if data["cpu_usage"].(float64) != 25.5 {
		t.Errorf("cpu_usage = %v, want 25.5", data["cpu_usage"])
	}
	if data["memory_percent"].(float64) != 60 {
		t.Errorf("memory_percent = %v, want 60", data["memory_percent"])
	}
	expected := map[string]float64{
		"cpu_steal":       0.2,
		"cpu_iowait":      0.1,
		"swap_total":      1024,
		"swap_used":       128,
		"swap_percent":    12.5,
		"net_rx_total":    10737418240,
		"net_tx_total":    5368709120,
		"uptime":          518400,
		"process_count":   128,
		"tcp_established": 42,
		"tcp_time_wait":   5,
	}
	for field, want := range expected {
		if data[field].(float64) != want {
			t.Errorf("%s = %v, want %v", field, data[field], want)
		}
	}
	disk := data["disk"].(map[string]interface{})
	if disk["/"].(map[string]interface{})["usage_percent"].(float64) != 65 {
		t.Errorf("unexpected disk data: %+v", disk)
	}
	network := data["network"].(map[string]interface{})
	if network["eth0"].(map[string]interface{})["rx_bytes"].(float64) != 1024 {
		t.Errorf("unexpected network data: %+v", network)
	}

	latency, ok := data["latency"].([]interface{})
	if !ok || len(latency) != 2 {
		t.Fatalf("unexpected latency data: %+v", data["latency"])
	}
	firstLatency := latency[0].(map[string]interface{})
	if firstLatency["target_name"] != "电信" ||
		firstLatency["target"] != "202.102.192.68:53" ||
		firstLatency["latency_ms"].(float64) != 42.5 ||
		firstLatency["jitter_ms"].(float64) != 2.1 ||
		firstLatency["packet_loss"].(float64) != 33.33 {
		t.Errorf("unexpected first latency item: %+v", firstLatency)
	}
	if latency[1].(map[string]interface{})["target_name"] != "联通" {
		t.Errorf("latency should be sorted by target_name: %+v", latency)
	}

	recorder = performRequest(t, api.router, http.MethodGet, "/api/agents/missing/metrics/realtime", nil)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("missing realtime status = %d, want 404", recorder.Code)
	}
}

func TestGetHistoryMetricsRaw(t *testing.T) {
	api := setupTestAPI(t)
	now := time.Now()

	// 创建上个月分区并写入跨月数据
	previousMonth := now.AddDate(0, -1, 0)
	if err := model.CreateMetricsRawPartitions(api.db, previousMonth, 1); err != nil {
		t.Fatalf("failed to create previous partition: %v", err)
	}
	previousTimestamp := time.Date(previousMonth.Year(), previousMonth.Month(), 15, 12, 0, 0, 0, time.Local).Unix()

	previousTable := model.GetMetricsRawTableName(time.Unix(previousTimestamp, 0))
	if err := api.db.Table(previousTable).Create(&model.MetricsRaw{
		AgentID:   "agent-1",
		Timestamp: previousTimestamp,
		CPUUsage:  10,
	}).Error; err != nil {
		t.Fatalf("failed to insert previous month metrics: %v", err)
	}

	currentTimestamp := now.Unix() - 60
	currentTable := model.GetMetricsRawTableName(now)
	if err := api.db.Table(currentTable).Create(&model.MetricsRaw{
		AgentID:   "agent-1",
		Timestamp: currentTimestamp,
		CPUUsage:  20,
	}).Error; err != nil {
		t.Fatalf("failed to insert current month metrics: %v", err)
	}

	start := previousTimestamp - 86400
	end := now.Unix() + 1
	path := "/api/agents/agent-1/metrics/history?metric=cpu&granularity=1m&start=" +
		strconv.FormatInt(start, 10) + "&end=" + strconv.FormatInt(end, 10)

	recorder := performRequest(t, api.router, http.MethodGet, path, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	points := data["points"].([]interface{})
	if len(points) != 2 {
		t.Fatalf("point count = %d, want 2: %+v", len(points), points)
	}
	if data["granularity"] != "1m" {
		t.Errorf("granularity = %v, want 1m", data["granularity"])
	}
	firstPoint := points[0].(map[string]interface{})
	secondPoint := points[1].(map[string]interface{})
	if firstPoint["value"].(float64) != 10 || secondPoint["value"].(float64) != 20 {
		t.Errorf("unexpected point values: %+v, %+v", firstPoint, secondPoint)
	}
	if firstPoint["timestamp"].(float64) >= secondPoint["timestamp"].(float64) {
		t.Error("points should be ordered by timestamp")
	}
}

func TestGetHistoryMetricsAggregated(t *testing.T) {
	api := setupTestAPI(t)
	now := time.Now().Unix()

	if err := api.db.Create(&model.MetricsAggregated{
		AgentID:     "agent-1",
		Granularity: "5m",
		Timestamp:   now - 300,
		CPUAvg:      30,
		CPUMax:      40,
		SampleCount: 5,
	}).Error; err != nil {
		t.Fatalf("failed to insert aggregated metrics: %v", err)
	}

	path := "/api/agents/agent-1/metrics/history?metric=cpu&granularity=5m&start=" +
		strconv.FormatInt(now-3600, 10) + "&end=" + strconv.FormatInt(now, 10)
	recorder := performRequest(t, api.router, http.MethodGet, path, nil)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	points := data["points"].([]interface{})
	if len(points) != 1 {
		t.Fatalf("point count = %d, want 1", len(points))
	}
	point := points[0].(map[string]interface{})
	if point["value"].(float64) != 30 {
		t.Errorf("value = %v, want 30", point["value"])
	}
}

func TestGetHistoryMetricsValidation(t *testing.T) {
	api := setupTestAPI(t)

	recorder := performRequest(t, api.router, http.MethodGet,
		"/api/agents/agent-1/metrics/history?metric=invalid", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid metric status = %d, want 400", recorder.Code)
	}

	recorder = performRequest(t, api.router, http.MethodGet,
		"/api/agents/agent-1/metrics/history?metric=cpu&granularity=bad", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid granularity status = %d, want 400", recorder.Code)
	}

	recorder = performRequest(t, api.router, http.MethodGet,
		"/api/agents/agent-1/metrics/history?metric=cpu&start=100&end=100", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("invalid range status = %d, want 400", recorder.Code)
	}

	recorder = performRequest(t, api.router, http.MethodGet,
		"/api/agents/agent-1/metrics/history?metric=disk&granularity=5m", nil)
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("disk granularity status = %d, want 400", recorder.Code)
	}
}

func TestAutoSelectGranularity(t *testing.T) {
	now := time.Now().Unix()
	tests := []struct {
		start int64
		end   int64
		want  string
	}{
		{now - 3600, now, "1m"},
		{now - 86400, now, "5m"},
		{now - 604800, now, "1h"},
		{now - 2592000, now, "1d"},
	}

	for _, test := range tests {
		if got := autoSelectGranularity(test.start, test.end); got != test.want {
			t.Errorf("autoSelectGranularity(%d, %d) = %q, want %q", test.start, test.end, got, test.want)
		}
	}
}
