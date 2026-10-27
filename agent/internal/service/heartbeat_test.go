package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/huang110/serverprobe/agent/internal/client"
	"github.com/huang110/serverprobe/agent/internal/collector"
	"github.com/huang110/serverprobe/agent/internal/config"
	pb "github.com/huang110/serverprobe/agent/proto"
)

// heartbeatTestServer 用于验证心跳请求和响应的本地服务端
type heartbeatTestServer struct {
	pb.UnimplementedProbeServiceServer

	mu       sync.Mutex
	requests []*pb.HeartbeatRequest
	response *pb.HeartbeatResponse
	err      error
}

func (s *heartbeatTestServer) Heartbeat(_ context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.requests = append(s.requests, req)
	return s.response, s.err
}

func (s *heartbeatTestServer) lastRequest() *pb.HeartbeatRequest {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.requests) == 0 {
		return nil
	}
	return s.requests[len(s.requests)-1]
}

func (s *heartbeatTestServer) requestCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.requests)
}

func (s *heartbeatTestServer) setResponse(response *pb.HeartbeatResponse) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.response = response
}

// startHeartbeatTestServer 启动本地 gRPC 服务并返回真实客户端
func startHeartbeatTestServer(t *testing.T, fake *heartbeatTestServer) *client.GRPCClient {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()
	pb.RegisterProbeServiceServer(server, fake)
	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	grpcClient, err := client.NewGRPCClient(
		&config.ServerConfig{Address: listener.Addr().String(), TLSMode: "none"},
		client.WithConnectTimeout(3*time.Second),
		client.WithRequestTimeout(5*time.Second),
		client.WithMaxConnectAttempts(1),
	)
	if err != nil {
		t.Fatalf("failed to create gRPC client: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := grpcClient.Close(); closeErr != nil {
			t.Errorf("failed to close gRPC client: %v", closeErr)
		}
	})

	return grpcClient
}

// newHeartbeatTestConfig 创建临时配置并加载
func newHeartbeatTestConfig(t *testing.T, address string) (string, *config.Config, *config.Loader) {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "agent.yaml")
	content := fmt.Sprintf(`agent:
  id: "agent-heartbeat"
  token: "token-1"
  token_expires_at: %d
  lite_mode: true
server:
  address: %q
  register_secret: "test-secret"
  tls_mode: "none"
monitor:
  heartbeat_interval: 30
  network_interfaces: []
  disk_mounts: []
  exclude_fs_types: []
labels:
  env: staging
ip_quality:
  enabled: false
  latency_targets: []
log:
  level: "error"
  file: ""
`, time.Now().Add(24*time.Hour).Unix(), address)

	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	loader := config.NewLoader(configPath)
	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("failed to load test config: %v", err)
	}
	return configPath, cfg, loader
}

// TestHeartbeatSendSuccess 验证心跳请求内容、签名和响应处理
func TestHeartbeatSendSuccess(t *testing.T) {
	newExpiresAt := time.Now().Add(48 * time.Hour).Unix()
	fake := &heartbeatTestServer{
		response: &pb.HeartbeatResponse{
			Success:               true,
			NewToken:              "token-2",
			NewTokenExpiresAt:     newExpiresAt,
			NextHeartbeatInterval: 15,
			ConfigUpdate: &pb.ConfigUpdate{
				HasUpdate:       true,
				Labels:          map[string]string{"env": "production", "region": "cn"},
				EnableIpQuality: true,
			},
		},
	}
	grpcClient := startHeartbeatTestServer(t, fake)

	configPath, cfg, loader := newHeartbeatTestConfig(t, "127.0.0.1:50051")
	svc := NewHeartbeatService(grpcClient, cfg, loader)

	before := time.Now().Unix()
	if err := svc.sendHeartbeat(); err != nil {
		t.Fatalf("sendHeartbeat() returned error: %v", err)
	}
	after := time.Now().Unix()

	request := fake.lastRequest()
	if request == nil {
		t.Fatal("server did not receive heartbeat request")
	}

	if request.AgentId != "agent-heartbeat" {
		t.Errorf("AgentId = %q, want %q", request.AgentId, "agent-heartbeat")
	}
	if request.Timestamp < before || request.Timestamp > after {
		t.Errorf("Timestamp %d is outside [%d, %d]", request.Timestamp, before, after)
	}
	expectedSignature := client.GenerateSignature(request.AgentId, request.Timestamp, "token-1")
	if request.Signature != expectedSignature {
		t.Errorf("Signature = %q, want %q", request.Signature, expectedSignature)
	}
	if request.Cpu == nil {
		t.Error("CPU metrics should not be nil")
	}
	if request.Memory == nil {
		t.Error("memory metrics should not be nil")
	}
	if request.System == nil {
		t.Error("system metrics should not be nil")
	}
	if request.AgentStatus == nil {
		t.Fatal("agent status should not be nil")
	}
	if request.AgentStatus.Goroutines <= 0 {
		t.Errorf("goroutines should be positive, got %d", request.AgentStatus.Goroutines)
	}
	if request.AgentStatus.Version != config.Version {
		t.Errorf("agent version = %q, want %q", request.AgentStatus.Version, config.Version)
	}
	if !request.AgentStatus.LiteMode {
		t.Error("agent status LiteMode should be true")
	}

	if cfg.Agent.Token != "token-2" || cfg.Agent.TokenExpiresAt != newExpiresAt {
		t.Errorf("token rotation not applied: %+v", cfg.Agent)
	}
	if cfg.Monitor.HeartbeatInterval != 15 {
		t.Errorf("heartbeat interval = %d, want 15", cfg.Monitor.HeartbeatInterval)
	}
	if got := svc.currentInterval(); got != 15*time.Second {
		t.Errorf("current interval = %s, want 15s", got)
	}
	if cfg.Labels["env"] != "production" || cfg.Labels["region"] != "cn" {
		t.Errorf("labels not updated: %+v", cfg.Labels)
	}
	if !cfg.IPQuality.Enabled {
		t.Error("IP quality should be enabled by config update")
	}

	saved := loadSavedConfig(t, configPath)
	if saved.Agent.Token != "token-2" || saved.Agent.TokenExpiresAt != newExpiresAt {
		t.Errorf("rotated token not persisted: %+v", saved.Agent)
	}
	if saved.Monitor.HeartbeatInterval != 15 {
		t.Errorf("heartbeat interval not persisted: %d", saved.Monitor.HeartbeatInterval)
	}
	if saved.Labels["env"] != "production" || saved.Labels["region"] != "cn" {
		t.Errorf("labels not persisted: %+v", saved.Labels)
	}
	if !saved.IPQuality.Enabled {
		t.Error("IP quality setting not persisted")
	}
}

// TestHeartbeatUsesRotatedToken 验证下一次心跳使用轮换后的 Token
func TestHeartbeatUsesRotatedToken(t *testing.T) {
	fake := &heartbeatTestServer{
		response: &pb.HeartbeatResponse{
			Success:           true,
			NewToken:          "token-2",
			NewTokenExpiresAt: time.Now().Add(48 * time.Hour).Unix(),
		},
	}
	grpcClient := startHeartbeatTestServer(t, fake)

	_, cfg, loader := newHeartbeatTestConfig(t, "127.0.0.1:50051")
	svc := NewHeartbeatService(grpcClient, cfg, loader)

	if err := svc.sendHeartbeat(); err != nil {
		t.Fatalf("first sendHeartbeat() returned error: %v", err)
	}

	fake.setResponse(&pb.HeartbeatResponse{Success: true})
	if err := svc.sendHeartbeat(); err != nil {
		t.Fatalf("second sendHeartbeat() returned error: %v", err)
	}

	request := fake.lastRequest()
	if request == nil {
		t.Fatal("server did not receive the second heartbeat")
	}
	expectedSignature := client.GenerateSignature(request.AgentId, request.Timestamp, "token-2")
	if request.Signature != expectedSignature {
		t.Errorf("second heartbeat did not use rotated token, signature = %q, want %q",
			request.Signature, expectedSignature)
	}
}

// TestHeartbeatRejected 验证服务端拒绝心跳时返回错误
func TestHeartbeatRejected(t *testing.T) {
	fake := &heartbeatTestServer{
		response: &pb.HeartbeatResponse{
			Success: false,
			Message: "invalid signature",
		},
	}
	grpcClient := startHeartbeatTestServer(t, fake)

	_, cfg, loader := newHeartbeatTestConfig(t, "127.0.0.1:50051")
	svc := NewHeartbeatService(grpcClient, cfg, loader)

	err := svc.sendHeartbeat()
	if err == nil {
		t.Fatal("sendHeartbeat() should return an error when server rejects the heartbeat")
	}
}

// TestHeartbeatStartStop 验证启动、重复启动、停止和重复停止
func TestHeartbeatStartStop(t *testing.T) {
	fake := &heartbeatTestServer{
		response: &pb.HeartbeatResponse{Success: true},
	}
	grpcClient := startHeartbeatTestServer(t, fake)

	_, cfg, loader := newHeartbeatTestConfig(t, "127.0.0.1:50051")
	svc := NewHeartbeatService(grpcClient, cfg, loader)

	if err := svc.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	if !svc.IsRunning() {
		t.Error("service should be running after Start()")
	}
	if err := svc.Start(); err == nil {
		t.Error("second Start() should return an error")
	}

	svc.Stop()
	if svc.IsRunning() {
		t.Error("service should not be running after Stop()")
	}
	svc.Stop()

	if err := svc.Start(); err == nil {
		t.Error("Start() after Stop() should return an error")
	}
}

// TestHeartbeatStopBeforeStart 验证未启动时 Stop 不会阻塞
func TestHeartbeatStopBeforeStart(t *testing.T) {
	svc := NewHeartbeatService(nil, nil, nil)
	svc.Stop()
	if svc.IsRunning() {
		t.Error("service should not be running")
	}
}

// TestHeartbeatBackoff 验证心跳失败后的指数退避
func TestHeartbeatBackoff(t *testing.T) {
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{0, 0},
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 16 * time.Second},
		{6, 32 * time.Second},
		{7, maxHeartbeatBackoff},
		{100, maxHeartbeatBackoff},
	}

	for _, test := range tests {
		if got := heartbeatBackoff(test.failures); got != test.want {
			t.Errorf("heartbeatBackoff(%d) = %s, want %s", test.failures, got, test.want)
		}
	}
}

// TestHeartbeatFailureCounter 验证失败计数和恢复后的调度间隔
func TestHeartbeatFailureCounter(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Monitor.HeartbeatInterval = 30
	svc := NewHeartbeatService(nil, cfg, nil)

	if delay := svc.nextHeartbeatDelay(); delay != 30*time.Second {
		t.Errorf("initial delay = %s, want 30s", delay)
	}

	if failures := svc.recordFailure(); failures != 1 {
		t.Errorf("failures = %d, want 1", failures)
	}
	if delay := svc.nextHeartbeatDelay(); delay != time.Second {
		t.Errorf("after first failure delay = %s, want 1s", delay)
	}

	svc.recordFailure()
	if delay := svc.nextHeartbeatDelay(); delay != 2*time.Second {
		t.Errorf("after second failure delay = %s, want 2s", delay)
	}

	svc.recordSuccess()
	if delay := svc.nextHeartbeatDelay(); delay != 30*time.Second {
		t.Errorf("after recovery delay = %s, want 30s", delay)
	}
}

// TestConvertMetrics 验证采集指标到 proto 指标的转换
func TestConvertMetrics(t *testing.T) {
	t.Run("CPU", func(t *testing.T) {
		got := convertCPUMetrics(&collector.CPUMetrics{
			UsagePercent:  12.5,
			UserPercent:   6,
			SystemPercent: 4,
			IdlePercent:   90,
			IowaitPercent: 1,
			StealPercent:  0.5,
			PerCoreUsage:  []float64{10, 20},
		})
		if got == nil || got.UsagePercent != 12.5 || got.UserPercent != 6 ||
			got.SystemPercent != 4 || got.IdlePercent != 90 || got.IowaitPercent != 1 ||
			got.StealPercent != 0.5 || len(got.PerCoreUsage) != 2 {
			t.Errorf("unexpected CPU conversion: %+v", got)
		}
	})

	t.Run("Memory", func(t *testing.T) {
		got := convertMemMetrics(&collector.MemoryMetrics{
			Total:            100,
			Used:             60,
			Free:             40,
			Available:        30,
			UsagePercent:     60,
			SwapTotal:        20,
			SwapUsed:         5,
			SwapFree:         15,
			SwapUsagePercent: 25,
			Buffers:          3,
			Cached:           4,
			Shared:           2,
		})
		if got == nil || got.Total != 100 || got.Used != 60 || got.Free != 40 ||
			got.Available != 30 || got.UsagePercent != 60 || got.SwapTotal != 20 ||
			got.SwapUsed != 5 || got.SwapFree != 15 || got.SwapUsagePercent != 25 ||
			got.Buffers != 3 || got.Cached != 4 || got.Shared != 2 {
			t.Errorf("unexpected memory conversion: %+v", got)
		}
	})

	t.Run("Disk", func(t *testing.T) {
		got := convertDiskMetrics([]collector.DiskMetrics{{
			Device:            "/dev/sda1",
			MountPoint:        "/",
			FsType:            "ext4",
			Total:             100,
			Used:              40,
			Free:              60,
			UsagePercent:      40,
			InodeTotal:        1000,
			InodeUsed:         100,
			InodeUsagePercent: 10,
			ReadBytesPerSec:   1024,
			WriteBytesPerSec:  2048,
			ReadIOPS:          10,
			WriteIOPS:         20,
			IOUtilPercent:     5,
		}})
		if len(got) != 1 || got[0].Device != "/dev/sda1" || got[0].MountPoint != "/" ||
			got[0].FsType != "ext4" || got[0].Total != 100 || got[0].Used != 40 ||
			got[0].Free != 60 || got[0].UsagePercent != 40 || got[0].InodeTotal != 1000 ||
			got[0].InodeUsed != 100 || got[0].InodeUsagePercent != 10 ||
			got[0].ReadBytesPerSec != 1024 || got[0].WriteBytesPerSec != 2048 ||
			got[0].ReadIops != 10 || got[0].WriteIops != 20 || got[0].IoUtilPercent != 5 {
			t.Errorf("unexpected disk conversion: %+v", got)
		}
	})

	t.Run("Network", func(t *testing.T) {
		got := convertNetMetrics([]collector.NetworkMetrics{{
			Interface:     "eth0",
			IPAddress:     "10.0.0.5",
			MACAddress:    "aa:bb:cc:dd:ee:ff",
			RxBytesPerSec: 100,
			TxBytesPerSec: 200,
			RxBytesTotal:  1000,
			TxBytesTotal:  2000,
			RxPackets:     10,
			TxPackets:     20,
			RxErrors:      1,
			TxErrors:      2,
			RxDropped:     3,
			TxDropped:     4,
		}})
		if len(got) != 1 || got[0].Interface != "eth0" || got[0].IpAddress != "10.0.0.5" ||
			got[0].MacAddress != "aa:bb:cc:dd:ee:ff" || got[0].RxBytesPerSec != 100 ||
			got[0].TxBytesPerSec != 200 || got[0].RxBytesTotal != 1000 ||
			got[0].TxBytesTotal != 2000 || got[0].RxPackets != 10 || got[0].TxPackets != 20 ||
			got[0].RxErrors != 1 || got[0].TxErrors != 2 || got[0].RxDropped != 3 ||
			got[0].TxDropped != 4 {
			t.Errorf("unexpected network conversion: %+v", got)
		}
	})

	t.Run("System", func(t *testing.T) {
		got := convertSysMetrics(&collector.SystemMetrics{
			Load1:          1,
			Load5:          2,
			Load15:         3,
			Uptime:         100,
			ProcessCount:   50,
			TcpEstablished: 10,
			TcpTimeWait:    5,
			TcpCloseWait:   2,
			UdpConnections: 7,
		})
		if got == nil || got.Load1 != 1 || got.Load5 != 2 || got.Load15 != 3 ||
			got.Uptime != 100 || got.ProcessCount != 50 || got.TcpEstablished != 10 ||
			got.TcpTimeWait != 5 || got.TcpCloseWait != 2 || got.UdpConnections != 7 {
			t.Errorf("unexpected system conversion: %+v", got)
		}
	})

	t.Run("Latency", func(t *testing.T) {
		got := convertLatencyMetrics([]collector.LatencyResult{
			{
				TargetName: "电信",
				Target:     "202.102.192.68:53",
				LatencyMs:  42.5,
				JitterMs:   2.1,
				PacketLoss: 33.33,
			},
		}, 1700000000)
		if got == nil || len(got.LatencyTests) != 1 || got.LastCheckTime != 1700000000 {
			t.Fatalf("unexpected latency conversion: %+v", got)
		}
		test := got.LatencyTests[0]
		if test.TargetName != "电信" || test.Target != "202.102.192.68:53" ||
			test.LatencyMs != 42.5 || test.JitterMs != 2.1 || test.PacketLoss != 33.33 {
			t.Errorf("unexpected latency test: %+v", test)
		}
	})

	t.Run("Nil", func(t *testing.T) {
		if convertCPUMetrics(nil) != nil || convertMemMetrics(nil) != nil ||
			convertSysMetrics(nil) != nil || convertDiskMetrics(nil) != nil ||
			convertNetMetrics(nil) != nil || convertLatencyMetrics(nil, 0) != nil {
			t.Error("nil metrics should convert to nil")
		}
	})
}
