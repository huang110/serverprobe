package cache

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// setupTestRedis 启动 miniredis 并创建客户端
func setupTestRedis(t *testing.T) (*RedisClient, *miniredis.Miniredis) {
	t.Helper()

	server := miniredis.RunT(t)
	client, err := NewRedisClient(server.Addr(), "", 0)
	if err != nil {
		t.Fatalf("NewRedisClient() failed: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("Close() failed: %v", closeErr)
		}
	})

	return client, server
}

func TestSaveAndGetRealtimeMetrics(t *testing.T) {
	client, server := setupTestRedis(t)

	metrics := map[string]interface{}{
		"cpu_usage":         12.5,
		"memory_percent":    60.0,
		"last_heartbeat_at": time.Now().Unix(),
	}
	if err := client.SaveRealtimeMetrics("agent-1", metrics); err != nil {
		t.Fatalf("SaveRealtimeMetrics() failed: %v", err)
	}

	got, err := client.GetRealtimeMetrics("agent-1")
	if err != nil {
		t.Fatalf("GetRealtimeMetrics() failed: %v", err)
	}
	if got["cpu_usage"] != "12.5" {
		t.Errorf("cpu_usage = %q, want %q", got["cpu_usage"], "12.5")
	}
	if got["memory_percent"] != "60" {
		t.Errorf("memory_percent = %q, want %q", got["memory_percent"], "60")
	}

	key := realtimeMetricsKey("agent-1")
	if keyType, _ := client.Client().Type(client.Context(), key).Result(); keyType != "hash" {
		t.Errorf("key type = %q, want hash", keyType)
	}
	if ttl := server.TTL(key); ttl <= 0 || ttl > realtimeMetricsTTL {
		t.Errorf("TTL = %s, want 0 < TTL <= %s", ttl, realtimeMetricsTTL)
	}
}

func TestOnlineAgents(t *testing.T) {
	client, _ := setupTestRedis(t)
	now := time.Now().Unix()

	if err := client.UpdateOnlineAgent("agent-1", now); err != nil {
		t.Fatalf("UpdateOnlineAgent() failed: %v", err)
	}
	if err := client.UpdateOnlineAgent("agent-2", now-100); err != nil {
		t.Fatalf("UpdateOnlineAgent() failed: %v", err)
	}
	if err := client.UpdateOnlineAgent("agent-3", now-200); err != nil {
		t.Fatalf("UpdateOnlineAgent() failed: %v", err)
	}

	agents, err := client.GetOnlineAgents(150)
	if err != nil {
		t.Fatalf("GetOnlineAgents() failed: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("GetOnlineAgents() = %v, want 2 agents", agents)
	}

	removed, err := client.RemoveOfflineAgents(150)
	if err != nil {
		t.Fatalf("RemoveOfflineAgents() failed: %v", err)
	}
	if removed != 1 {
		t.Errorf("RemoveOfflineAgents() removed %d, want 1", removed)
	}

	agents, err = client.GetOnlineAgents(150)
	if err != nil {
		t.Fatalf("GetOnlineAgents() after cleanup failed: %v", err)
	}
	if len(agents) != 2 {
		t.Errorf("online agents after cleanup = %v, want 2", agents)
	}
}

func TestSaveAndGetDiskMetrics(t *testing.T) {
	client, server := setupTestRedis(t)

	disks := map[string]interface{}{
		"/": map[string]interface{}{
			"usage_percent": 45.5,
			"read_bytes":    1024.0,
			"write_bytes":   2048.0,
			"io_util":       3.2,
		},
	}
	if err := client.SaveDiskMetrics("agent-1", disks); err != nil {
		t.Fatalf("SaveDiskMetrics() failed: %v", err)
	}

	got, err := client.GetDiskMetrics("agent-1")
	if err != nil {
		t.Fatalf("GetDiskMetrics() failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("disk metric count = %d, want 1", len(got))
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got["/"]), &decoded); err != nil {
		t.Fatalf("failed to unmarshal disk metric: %v", err)
	}
	if decoded["usage_percent"] != 45.5 {
		t.Errorf("usage_percent = %v, want 45.5", decoded["usage_percent"])
	}

	key := realtimeDiskKey("agent-1")
	if ttl := server.TTL(key); ttl <= 0 {
		t.Errorf("disk metric TTL = %s, want > 0", ttl)
	}

	// 保存空集合应清空旧数据
	if err := client.SaveDiskMetrics("agent-1", nil); err != nil {
		t.Fatalf("SaveDiskMetrics(nil) failed: %v", err)
	}
	got, _ = client.GetDiskMetrics("agent-1")
	if len(got) != 0 {
		t.Errorf("disk metrics should be empty after saving nil, got %v", got)
	}
}

func TestSaveAndGetNetworkMetrics(t *testing.T) {
	client, _ := setupTestRedis(t)

	networks := map[string]interface{}{
		"eth0": map[string]interface{}{
			"rx_bytes":  100,
			"tx_bytes":  200,
			"rx_errors": 1,
			"tx_errors": 2,
		},
	}
	if err := client.SaveNetworkMetrics("agent-1", networks); err != nil {
		t.Fatalf("SaveNetworkMetrics() failed: %v", err)
	}

	got, err := client.GetNetworkMetrics("agent-1")
	if err != nil {
		t.Fatalf("GetNetworkMetrics() failed: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("network metric count = %d, want 1", len(got))
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(got["eth0"]), &decoded); err != nil {
		t.Fatalf("failed to unmarshal network metric: %v", err)
	}
	if decoded["rx_bytes"] != float64(100) || decoded["tx_errors"] != float64(2) {
		t.Errorf("unexpected network metric: %+v", decoded)
	}
}

func TestAgentDetailCache(t *testing.T) {
	client, server := setupTestRedis(t)

	type detail struct {
		ID       string `json:"id"`
		Hostname string `json:"hostname"`
	}
	input := detail{ID: "agent-1", Hostname: "node-1"}
	if err := client.SaveAgentDetail("agent-1", input); err != nil {
		t.Fatalf("SaveAgentDetail() failed: %v", err)
	}

	var output detail
	found, err := client.GetAgentDetail("agent-1", &output)
	if err != nil {
		t.Fatalf("GetAgentDetail() failed: %v", err)
	}
	if !found || output.Hostname != "node-1" {
		t.Errorf("unexpected agent detail: found=%v output=%+v", found, output)
	}

	if ttl := server.TTL(agentDetailKey("agent-1")); ttl <= 0 || ttl > agentDetailTTL {
		t.Errorf("agent detail TTL = %s, want 0 < TTL <= %s", ttl, agentDetailTTL)
	}

	var missing detail
	found, err = client.GetAgentDetail("agent-2", &missing)
	if err != nil {
		t.Fatalf("GetAgentDetail() for missing key failed: %v", err)
	}
	if found {
		t.Error("missing agent detail should return found=false")
	}
}

func TestAlertStateCache(t *testing.T) {
	client, _ := setupTestRedis(t)

	state := map[string]interface{}{
		"first_trigger_at":  1700000000,
		"consecutive_count": 3,
		"last_check_at":     1700000060,
	}
	if err := client.SaveAlertState(1, "agent-1", state); err != nil {
		t.Fatalf("SaveAlertState() failed: %v", err)
	}

	got, err := client.GetAlertState(1, "agent-1")
	if err != nil {
		t.Fatalf("GetAlertState() failed: %v", err)
	}
	if got["consecutive_count"] != "3" || got["first_trigger_at"] != "1700000000" {
		t.Errorf("unexpected alert state: %+v", got)
	}

	if err := client.DeleteAlertState(1, "agent-1"); err != nil {
		t.Fatalf("DeleteAlertState() failed: %v", err)
	}
	got, err = client.GetAlertState(1, "agent-1")
	if err != nil {
		t.Fatalf("GetAlertState() after delete failed: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("alert state should be empty after delete, got %+v", got)
	}
}

func TestRealtimeValidation(t *testing.T) {
	client, _ := setupTestRedis(t)

	if err := client.SaveRealtimeMetrics("", nil); err == nil {
		t.Error("empty agent id should return an error")
	}
	if _, err := client.GetOnlineAgents(0); err == nil {
		t.Error("zero threshold should return an error")
	}
	if _, err := client.RemoveOfflineAgents(-1); err == nil {
		t.Error("negative threshold should return an error")
	}
	if err := client.SaveDiskMetrics("", nil); err == nil {
		t.Error("empty agent id should return an error")
	}
	if err := client.SaveNetworkMetrics("", nil); err == nil {
		t.Error("empty agent id should return an error")
	}
}

func TestDeleteAgentCache(t *testing.T) {
	client, _ := setupTestRedis(t)

	if err := client.SaveRealtimeMetrics("agent-1", map[string]interface{}{"cpu_usage": 1}); err != nil {
		t.Fatalf("SaveRealtimeMetrics() failed: %v", err)
	}
	if err := client.SaveDiskMetrics("agent-1", map[string]interface{}{"/": map[string]interface{}{"usage_percent": 1}}); err != nil {
		t.Fatalf("SaveDiskMetrics() failed: %v", err)
	}
	if err := client.SaveNetworkMetrics("agent-1", map[string]interface{}{"eth0": map[string]interface{}{"rx_bytes": 1}}); err != nil {
		t.Fatalf("SaveNetworkMetrics() failed: %v", err)
	}
	if err := client.SaveAgentDetail("agent-1", map[string]string{"hostname": "node-1"}); err != nil {
		t.Fatalf("SaveAgentDetail() failed: %v", err)
	}
	if err := client.UpdateOnlineAgent("agent-1", time.Now().Unix()); err != nil {
		t.Fatalf("UpdateOnlineAgent() failed: %v", err)
	}

	if err := client.DeleteAgentCache("agent-1"); err != nil {
		t.Fatalf("DeleteAgentCache() failed: %v", err)
	}

	realtime, _ := client.GetRealtimeMetrics("agent-1")
	disk, _ := client.GetDiskMetrics("agent-1")
	network, _ := client.GetNetworkMetrics("agent-1")
	if len(realtime) != 0 || len(disk) != 0 || len(network) != 0 {
		t.Errorf("agent metric caches should be empty: realtime=%v disk=%v network=%v", realtime, disk, network)
	}

	var detail map[string]string
	found, err := client.GetAgentDetail("agent-1", &detail)
	if err != nil {
		t.Fatalf("GetAgentDetail() failed: %v", err)
	}
	if found {
		t.Error("agent detail cache should be deleted")
	}

	online, err := client.GetOnlineAgents(300)
	if err != nil {
		t.Fatalf("GetOnlineAgents() failed: %v", err)
	}
	if len(online) != 0 {
		t.Errorf("online set should be empty, got %v", online)
	}
}
