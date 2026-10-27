package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/huang110/serverprobe/server/proto"
)

const (
	registerSecret = "integration-register-secret"
	jwtSecret      = "integration-jwt-secret"
	adminUsername  = "admin"
	adminPassword  = "admin123"
)

var (
	baseURL     string
	grpcAddress string
	redisServer *miniredis.Miniredis
	logPath     string
)

func TestMain(m *testing.M) {
	tempDir, err := os.MkdirTemp("", "serverprobe-integration-*")
	if err != nil {
		fmt.Printf("failed to create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(tempDir)

	serverDir, err := filepath.Abs("../server")
	if err != nil {
		fmt.Printf("failed to resolve server dir: %v\n", err)
		os.Exit(1)
	}

	binary := filepath.Join(tempDir, "serverprobe-server")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/server")
	build.Dir = serverDir
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		fmt.Printf("failed to build server: %v\n%s\n", buildErr, output)
		os.Exit(1)
	}

	redisServer, err = miniredis.Run()
	if err != nil {
		fmt.Printf("failed to start miniredis: %v\n", err)
		os.Exit(1)
	}
	defer redisServer.Close()

	httpPort := freePort()
	grpcPort := freePort()
	baseURL = fmt.Sprintf("http://127.0.0.1:%d", httpPort)
	grpcAddress = fmt.Sprintf("127.0.0.1:%d", grpcPort)

	dbPath := filepath.ToSlash(filepath.Join(tempDir, "integration.db"))
	configPath := filepath.Join(tempDir, "config.yaml")
	configContent := fmt.Sprintf(`
server:
  http_address: "127.0.0.1:%d"
  grpc_address: "127.0.0.1:%d"
  mode: "release"
database:
  driver: "sqlite"
  dsn: "%s"
  max_open_conns: 5
  max_idle_conns: 2
redis:
  addr: "%s"
  password: ""
  db: 0
agent:
  register_secret: "%s"
  heartbeat_interval: 5
  token_expires_days: 7
jwt:
  secret: "%s"
  admin_username: "%s"
  admin_password: "%s"
alert:
  check_interval: 1
aggregation:
  enabled: true
  interval_seconds: 1
  include_current_bucket: true
cors:
  allowed_origins:
    - "*"
rate_limit:
  enabled: false
`, httpPort, grpcPort, dbPath, redisServer.Addr(), registerSecret, jwtSecret, adminUsername, adminPassword)

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		fmt.Printf("failed to write config: %v\n", err)
		os.Exit(1)
	}

	logPath = filepath.Join(tempDir, "server.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		fmt.Printf("failed to create log file: %v\n", err)
		os.Exit(1)
	}
	defer logFile.Close()

	command := exec.Command(binary, "-config", configPath)
	command.Stdout = logFile
	command.Stderr = logFile
	if err := command.Start(); err != nil {
		fmt.Printf("failed to start server: %v\n", err)
		os.Exit(1)
	}

	if err := waitForServer(20 * time.Second); err != nil {
		if data, readErr := os.ReadFile(logPath); readErr == nil {
			fmt.Printf("server log:\n%s\n", data)
		}
		_ = command.Process.Kill()
		_, _ = command.Process.Wait()
		fmt.Printf("server did not become ready: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	_ = command.Process.Kill()
	_, _ = command.Process.Wait()
	os.Exit(code)
}

func freePort() int {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForServer(timeout time.Duration) error {
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		response, err := client.Get(baseURL + "/api/health/ready")
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", baseURL)
}

func dialGRPC(t *testing.T) *grpc.ClientConn {
	t.Helper()

	conn, err := grpc.Dial(grpcAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("failed to dial gRPC: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := conn.Close(); closeErr != nil {
			t.Errorf("failed to close gRPC connection: %v", closeErr)
		}
	})
	return conn
}

func login(t *testing.T) string {
	t.Helper()

	body := []byte(fmt.Sprintf(`{"username":%q,"password":%q}`, adminUsername, adminPassword))
	status, response := doJSON(t, http.MethodPost, "/api/auth/login", "", body)
	if status != http.StatusOK {
		t.Fatalf("login status = %d, response=%+v", status, response)
	}
	data, _ := response["data"].(map[string]interface{})
	token, _ := data["token"].(string)
	if token == "" {
		t.Fatalf("login did not return a token: %+v", response)
	}
	return token
}

func doJSON(t *testing.T, method, path, token string, body []byte) (int, map[string]interface{}) {
	t.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	request, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request %s %s failed: %v", method, path, err)
	}
	defer response.Body.Close()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("failed to read response: %v", err)
	}

	var decoded map[string]interface{}
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &decoded)
	}
	return response.StatusCode, decoded
}

func registerAgent(t *testing.T, client pb.ProbeServiceClient, hostname string) *pb.RegisterResponse {
	t.Helper()

	response, err := client.Register(context.Background(), &pb.RegisterRequest{
		RegisterSecret: registerSecret,
		Hostname:       hostname,
		Ip:             "203.0.113.10",
		InternalIp:     "10.0.0.5",
		Os:             runtime.GOOS,
		OsVersion:      "integration",
		KernelVersion:  "test",
		CpuCores:       2,
		MemoryTotal:    4096,
		DiskTotal:      10240,
		Arch:           runtime.GOARCH,
		AgentVersion:   "1.0.0",
		Labels:         map[string]string{"env": "test"},
	})
	if err != nil {
		t.Fatalf("Register() failed: %v", err)
	}
	if !response.Success || response.AgentId == "" || response.Token == "" {
		t.Fatalf("Register() returned invalid response: %+v", response)
	}
	return response
}

func signature(agentID string, timestamp int64, token string) string {
	message := fmt.Sprintf("%s:%d", agentID, timestamp)
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

func sendHeartbeat(t *testing.T, client pb.ProbeServiceClient, agentID, token string, timestamp int64, cpu float64) *pb.HeartbeatResponse {
	t.Helper()

	response, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
		AgentId:   agentID,
		Timestamp: timestamp,
		Signature: signature(agentID, timestamp, token),
		Cpu: &pb.CPUMetrics{
			UsagePercent:  cpu,
			StealPercent:  0.2,
			IowaitPercent: 0.1,
		},
		Memory: &pb.MemoryMetrics{
			Used:         2048,
			UsagePercent: 50,
			SwapTotal:    1024,
			SwapUsed:     128,
		},
		System: &pb.SystemMetrics{
			Load1:          1.2,
			Uptime:         518400,
			ProcessCount:   128,
			TcpEstablished: 3,
			TcpTimeWait:    2,
		},
		Disks: []*pb.DiskMetrics{{MountPoint: "/", UsagePercent: 50}},
		Networks: []*pb.NetworkMetrics{{
			Interface:     "eth0",
			RxBytesPerSec: 100,
			TxBytesPerSec: 50,
			RxBytesTotal:  10737418240,
			TxBytesTotal:  5368709120,
		}},
	})
	if err != nil {
		t.Fatalf("Heartbeat() failed: %v", err)
	}
	if !response.Success {
		t.Fatalf("Heartbeat() rejected: %s", response.Message)
	}
	return response
}

func Test18AgentRegistration(t *testing.T) {
	conn := dialGRPC(t)
	client := pb.NewProbeServiceClient(conn)
	response := registerAgent(t, client, "integration-register")

	token := login(t)
	status, body := doJSON(t, http.MethodGet, "/api/agents", token, nil)
	if status != http.StatusOK {
		t.Fatalf("GET /api/agents status = %d", status)
	}
	data, _ := body["data"].(map[string]interface{})
	agents, _ := data["agents"].([]interface{})
	found := false
	for _, item := range agents {
		agent, _ := item.(map[string]interface{})
		if agent["id"] == response.AgentId {
			found = true
			if agent["ip"] != "203.0.113.10" || agent["internal_ip"] != "10.0.0.5" {
				t.Errorf("authenticated agent should expose full IP: %+v", agent)
			}
			break
		}
	}
	if !found {
		t.Errorf("registered agent %s not found in agent list", response.AgentId)
	}

	// 游客模式：不携带 JWT，IP 必须脱敏
	status, body = doJSON(t, http.MethodGet, "/api/agents", "", nil)
	if status != http.StatusOK {
		t.Fatalf("guest GET /api/agents status = %d", status)
	}
	data, _ = body["data"].(map[string]interface{})
	agents, _ = data["agents"].([]interface{})
	for _, item := range agents {
		agent, _ := item.(map[string]interface{})
		if agent["id"] == response.AgentId {
			if agent["ip"] != "203.0.*.*" {
				t.Errorf("guest ip = %v, want 203.0.*.*", agent["ip"])
			}
			if agent["internal_ip"] != "" {
				t.Errorf("guest internal_ip = %v, want empty", agent["internal_ip"])
			}
			return
		}
	}
	t.Errorf("registered agent %s not found in guest list", response.AgentId)
}

func Test19HeartbeatRealtimeAndWebSocket(t *testing.T) {
	conn := dialGRPC(t)
	client := pb.NewProbeServiceClient(conn)
	agent := registerAgent(t, client, "integration-heartbeat")
	token := login(t)

	wsURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/ws?token=" + url.QueryEscape(token)
	socket, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer socket.Close()

	timestamp := time.Now().Unix()
	sendHeartbeat(t, client, agent.AgentId, agent.Token, timestamp, 25.5)

	message := readWebSocketMessage(t, socket, "realtime_metrics", agent.AgentId, 6*time.Second)
	data, _ := message["data"].(map[string]interface{})
	if data["agent_id"] != agent.AgentId || data["cpu_usage"].(float64) != 25.5 {
		t.Errorf("unexpected realtime message: %+v", data)
	}

	status, body := doJSON(t, http.MethodGet, "/api/agents/"+agent.AgentId+"/metrics/realtime", token, nil)
	if status != http.StatusOK {
		t.Fatalf("realtime status = %d, body=%+v", status, body)
	}
	realtime, _ := body["data"].(map[string]interface{})
	if realtime["cpu_usage"].(float64) != 25.5 {
		t.Errorf("realtime cpu = %v, want 25.5", realtime["cpu_usage"])
	}
	expectedRealtime := map[string]float64{
		"net_rx_total":    10737418240,
		"net_tx_total":    5368709120,
		"uptime":          518400,
		"process_count":   128,
		"cpu_steal":       0.2,
		"cpu_iowait":      0.1,
		"swap_total":      1024,
		"swap_used":       128,
		"tcp_established": 3,
		"tcp_time_wait":   2,
	}
	for field, want := range expectedRealtime {
		if realtime[field].(float64) != want {
			t.Errorf("realtime %s = %v, want %v", field, realtime[field], want)
		}
	}

	start := timestamp - 3600
	end := timestamp + 1
	path := fmt.Sprintf("/api/agents/%s/metrics/history?metric=cpu&granularity=1m&start=%d&end=%d", agent.AgentId, start, end)
	status, body = doJSON(t, http.MethodGet, path, token, nil)
	if status != http.StatusOK {
		t.Fatalf("history status = %d", status)
	}
	data, _ = body["data"].(map[string]interface{})
	points, _ := data["points"].([]interface{})
	if len(points) == 0 {
		t.Error("expected raw history points")
	}

	foundKey := false
	for _, key := range redisServer.Keys() {
		if strings.HasPrefix(key, "metrics:realtime:"+agent.AgentId) {
			foundKey = true
			break
		}
	}
	if !foundKey {
		t.Errorf("redis realtime key for agent %s not found", agent.AgentId)
	}
}

func Test20Aggregation(t *testing.T) {
	conn := dialGRPC(t)
	client := pb.NewProbeServiceClient(conn)
	agent := registerAgent(t, client, "integration-aggregation")
	token := login(t)

	now := time.Now().UTC().Unix()
	windowStart := alignTo(now, 300)
	windowEnd := windowStart + 300

	for i := 0; i < 10; i++ {
		sendHeartbeat(t, client, agent.AgentId, agent.Token, now, 10)
	}
	for i := 0; i < 10; i++ {
		sendHeartbeat(t, client, agent.AgentId, agent.Token, now, 30)
	}

	path := fmt.Sprintf("/api/agents/%s/metrics/history?metric=cpu&granularity=5m&start=%d&end=%d",
		agent.AgentId, windowStart, windowEnd)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		status, body := doJSON(t, http.MethodGet, path, token, nil)
		if status == http.StatusOK {
			data, _ := body["data"].(map[string]interface{})
			points, _ := data["points"].([]interface{})
			if len(points) > 0 {
				point, _ := points[0].(map[string]interface{})
				if point["value"].(float64) == 20 {
					return
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatal("5m aggregation did not produce expected average of 20")
}

func Test21AlertFiringAndResolved(t *testing.T) {
	conn := dialGRPC(t)
	client := pb.NewProbeServiceClient(conn)
	agent := registerAgent(t, client, "integration-alert")
	token := login(t)

	webhookCh := make(chan map[string]interface{}, 4)
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err == nil {
			webhookCh <- payload
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer webhook.Close()

	ruleBody, _ := json.Marshal(map[string]interface{}{
		"name":        "Integration CPU High",
		"agent_id":    agent.AgentId,
		"metric":      "cpu_usage",
		"condition":   "gt",
		"threshold":   80,
		"duration":    1,
		"severity":    "critical",
		"webhook_url": webhook.URL,
	})
	status, body := doJSON(t, http.MethodPost, "/api/alert/rules", token, ruleBody)
	if status != http.StatusOK {
		t.Fatalf("create rule status = %d, body=%+v", status, body)
	}
	ruleID := uint64(body["data"].(map[string]interface{})["id"].(float64))

	updateBody, _ := json.Marshal(map[string]interface{}{
		"name":     "Integration CPU Critical",
		"duration": 2,
		"enabled":  true,
	})
	status, body = doJSON(t, http.MethodPut, fmt.Sprintf("/api/alert/rules/%d", ruleID), token, updateBody)
	if status != http.StatusOK {
		t.Fatalf("update rule status = %d, body=%+v", status, body)
	}
	updatedRule := body["data"].(map[string]interface{})
	if updatedRule["name"] != "Integration CPU Critical" || updatedRule["duration"].(float64) != 2 {
		t.Errorf("rule update not applied: %+v", updatedRule)
	}

	wsURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/ws?token=" + url.QueryEscape(token)
	socket, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to connect websocket: %v", err)
	}
	defer socket.Close()

	sendHeartbeat(t, client, agent.AgentId, agent.Token, time.Now().Unix(), 95)

	if !waitForRecordStatus(t, token, agent.AgentId, "firing", 10*time.Second) {
		t.Fatal("alert did not enter firing state")
	}

	select {
	case payload := <-webhookCh:
		if payload["status"] != "firing" {
			t.Errorf("webhook status = %v, want firing", payload["status"])
		}
	case <-time.After(3 * time.Second):
		t.Error("webhook firing notification not received")
	}

	message := readWebSocketMessage(t, socket, "alert", agent.AgentId, 6*time.Second)
	data, _ := message["data"].(map[string]interface{})
	if data["agent_id"] != agent.AgentId {
		t.Errorf("unexpected alert message: %+v", data)
	}

	sendHeartbeat(t, client, agent.AgentId, agent.Token, time.Now().Unix(), 50)
	if !waitForRecordStatus(t, token, agent.AgentId, "resolved", 10*time.Second) {
		t.Fatal("alert did not resolve")
	}

	status, body = doJSON(t, http.MethodDelete, fmt.Sprintf("/api/alert/rules/%d", ruleID), token, nil)
	if status != http.StatusOK {
		t.Fatalf("delete rule status = %d, body=%+v", status, body)
	}
	if body["message"] != "rule deleted" {
		t.Errorf("unexpected delete response: %+v", body)
	}
}

func Test22CustomScripts(t *testing.T) {
	t.Skip("custom script metrics are not implemented in the current Agent/proto")
}

func Test23WebSocketReconnect(t *testing.T) {
	conn := dialGRPC(t)
	client := pb.NewProbeServiceClient(conn)
	agent := registerAgent(t, client, "integration-reconnect")
	token := login(t)

	wsURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/ws?token=" + url.QueryEscape(token)
	first, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("first websocket connection failed: %v", err)
	}
	_ = first.Close()

	second, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("reconnect failed: %v", err)
	}
	defer second.Close()

	sendHeartbeat(t, client, agent.AgentId, agent.Token, time.Now().Unix(), 12.5)
	message := readWebSocketMessage(t, second, "realtime_metrics", agent.AgentId, 6*time.Second)
	data, _ := message["data"].(map[string]interface{})
	if data["agent_id"] != agent.AgentId {
		t.Errorf("unexpected message after reconnect: %+v", data)
	}

	// 游客 WebSocket：无 token 允许连接，仅用于只读推送
	guestURL := strings.Replace(baseURL, "http://", "ws://", 1) + "/ws"
	guestSocket, _, err := websocket.DefaultDialer.Dial(guestURL, nil)
	if err != nil {
		t.Fatalf("guest websocket connection failed: %v", err)
	}
	defer guestSocket.Close()
	if err := guestSocket.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)); err != nil {
		t.Fatalf("failed to send guest ping: %v", err)
	}
	pong := readWebSocketMessage(t, guestSocket, "pong", "", 6*time.Second)
	if pong["type"] != "pong" {
		t.Errorf("unexpected guest websocket response: %+v", pong)
	}
}

func Test24ConcurrentAgents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping concurrency test in short mode")
	}

	conn := dialGRPC(t)
	client := pb.NewProbeServiceClient(conn)

	const agentCount = 10
	var wg sync.WaitGroup
	errorsCh := make(chan error, agentCount)

	for i := 0; i < agentCount; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			hostname := fmt.Sprintf("integration-load-%d-%d", time.Now().UnixNano(), index)
			response, err := client.Register(context.Background(), &pb.RegisterRequest{
				RegisterSecret: registerSecret,
				Hostname:       hostname,
				Os:             "linux",
				CpuCores:       2,
			})
			if err != nil || !response.Success {
				errorsCh <- fmt.Errorf("register %s failed: %v", hostname, err)
				return
			}

			timestamp := time.Now().Unix()
			heartbeat, err := client.Heartbeat(context.Background(), &pb.HeartbeatRequest{
				AgentId:   response.AgentId,
				Timestamp: timestamp,
				Signature: signature(response.AgentId, timestamp, response.Token),
				Cpu:       &pb.CPUMetrics{UsagePercent: 10},
				Memory:    &pb.MemoryMetrics{Used: 1024},
				System:    &pb.SystemMetrics{Load1: 1},
			})
			if err != nil || !heartbeat.Success {
				errorsCh <- fmt.Errorf("heartbeat %s failed: %v", hostname, err)
			}
		}(i)
	}

	wg.Wait()
	close(errorsCh)
	for err := range errorsCh {
		t.Error(err)
	}
}

func Test25FrontendSmoke(t *testing.T) {
	t.Skip("frontend flow is covered by frontend/e2e/playwright tests; browser execution is environment-dependent")
}

func readWebSocketMessage(t *testing.T, socket *websocket.Conn, messageType, agentID string, timeout time.Duration) map[string]interface{} {
	t.Helper()

	_ = socket.SetReadDeadline(time.Now().Add(timeout))
	for {
		_, payload, err := socket.ReadMessage()
		if err != nil {
			t.Fatalf("failed to read websocket message: %v", err)
		}
		var message map[string]interface{}
		if err := json.Unmarshal(payload, &message); err != nil {
			continue
		}
		if message["type"] != messageType {
			continue
		}
		if agentID != "" {
			data, _ := message["data"].(map[string]interface{})
			if data["agent_id"] != agentID {
				continue
			}
		}
		return message
	}
}

func waitForRecordStatus(t *testing.T, token, agentID, status string, timeout time.Duration) bool {
	t.Helper()

	deadline := time.Now().Add(timeout)
	path := fmt.Sprintf("/api/alert/records?agent_id=%s&status=%s", agentID, status)
	for time.Now().Before(deadline) {
		code, body := doJSON(t, http.MethodGet, path, token, nil)
		if code == http.StatusOK {
			data, _ := body["data"].([]interface{})
			if len(data) > 0 {
				return true
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	return false
}

func alignTo(timestamp, window int64) int64 {
	return timestamp - timestamp%window
}
