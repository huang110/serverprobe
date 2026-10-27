package websocket

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/huang110/serverprobe/server/internal/cache"
	"github.com/huang110/serverprobe/server/internal/model"
)

func waitForClientCount(t *testing.T, hub *Hub, want int) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if hub.ClientCount() == want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("client count = %d, want %d", hub.ClientCount(), want)
}

func TestHubRegisterUnregisterBroadcast(t *testing.T) {
	hub := NewHub()
	go hub.Run()
	defer hub.Stop()

	first := &Client{send: make(chan []byte, 4)}
	second := &Client{send: make(chan []byte, 4)}

	hub.Register(first)
	hub.Register(second)
	waitForClientCount(t, hub, 2)

	hub.Broadcast([]byte("hello"))
	for index, client := range []*Client{first, second} {
		select {
		case message := <-client.send:
			if string(message) != "hello" {
				t.Errorf("client %d message = %q, want hello", index, message)
			}
		case <-time.After(time.Second):
			t.Fatalf("client %d did not receive message", index)
		}
	}

	hub.Unregister(first)
	waitForClientCount(t, hub, 1)

	select {
	case _, ok := <-first.send:
		if ok {
			t.Error("unregistered client send channel should be closed")
		}
	case <-time.After(time.Second):
		t.Error("unregistered client send channel was not closed")
	}

	hub.Broadcast([]byte("world"))
	select {
	case message := <-second.send:
		if string(message) != "world" {
			t.Errorf("second client message = %q, want world", message)
		}
	case <-time.After(time.Second):
		t.Error("second client did not receive message after first unregistered")
	}
}

func TestBuildRealtimeMessage(t *testing.T) {
	payload := buildRealtimeMessage("agent-1", map[string]string{
		"cpu_usage":      "25.5",
		"cpu_steal":      "0.2",
		"memory_percent": "60",
		"swap_percent":   "12.5",
		"net_rx_bytes":   "1024",
		"net_tx_bytes":   "512",
		"net_rx_total":   "10737418240",
		"net_tx_total":   "5368709120",
		"uptime":         "518400",
	}, 1700000000, []model.LatencyItem{
		{TargetName: "电信", Target: "202.102.192.68:53", LatencyMs: 42.5, JitterMs: 2.1},
		{TargetName: "联通", Target: "210.22.70.3:53", LatencyMs: 48.2, JitterMs: 3.4, PacketLoss: 33.33},
	})

	var message map[string]interface{}
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}
	if message["type"] != "realtime_metrics" {
		t.Errorf("type = %v, want realtime_metrics", message["type"])
	}
	data := message["data"].(map[string]interface{})
	if data["agent_id"] != "agent-1" || data["cpu_usage"].(float64) != 25.5 {
		t.Errorf("unexpected data: %+v", data)
	}
	expected := map[string]float64{
		"cpu_steal":    0.2,
		"swap_percent": 12.5,
		"net_rx_total": 10737418240,
		"net_tx_total": 5368709120,
		"uptime":       518400,
	}
	for field, want := range expected {
		if data[field].(float64) != want {
			t.Errorf("%s = %v, want %v", field, data[field], want)
		}
	}

	latency := data["latency"].([]interface{})
	if len(latency) != 2 {
		t.Fatalf("latency length = %d, want 2", len(latency))
	}
	first := latency[0].(map[string]interface{})
	if first["target_name"] != "电信" || first["target"] != "202.102.192.68:53" ||
		first["latency_ms"].(float64) != 42.5 || first["jitter_ms"].(float64) != 2.1 ||
		first["packet_loss"].(float64) != 0 {
		t.Errorf("unexpected latency item: %+v", first)
	}
}

func TestBuildRealtimeMessageEmptyLatency(t *testing.T) {
	payload := buildRealtimeMessage("agent-1", map[string]string{"cpu_usage": "1"}, 1700000000, nil)

	var message map[string]interface{}
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}
	data := message["data"].(map[string]interface{})
	latency, ok := data["latency"].([]interface{})
	if !ok || len(latency) != 0 {
		t.Errorf("latency = %#v, want empty array", data["latency"])
	}
}

func TestBuildAgentStatusMessage(t *testing.T) {
	payload := buildAgentStatusMessage("agent-1", "online", 1700000000)

	var message map[string]interface{}
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}
	if message["type"] != "agent_status" {
		t.Errorf("type = %v, want agent_status", message["type"])
	}
	data := message["data"].(map[string]interface{})
	if data["status"] != "online" || data["last_heartbeat_at"].(float64) != 1700000000 {
		t.Errorf("unexpected data: %+v", data)
	}
}

func TestBuildAlertMessage(t *testing.T) {
	payload := buildAlertMessage(cache.AgentEvent{
		Type:     cache.EventTypeAlert,
		AgentID:  "agent-1",
		RuleID:   7,
		Severity: "critical",
		Message:  "CPU high",
		FiredAt:  1700000000,
	})

	var message map[string]interface{}
	if err := json.Unmarshal(payload, &message); err != nil {
		t.Fatalf("failed to unmarshal message: %v", err)
	}
	if message["type"] != "alert" {
		t.Errorf("type = %v, want alert", message["type"])
	}
	data := message["data"].(map[string]interface{})
	if data["rule_id"].(float64) != 7 || data["severity"] != "critical" {
		t.Errorf("unexpected data: %+v", data)
	}
}
