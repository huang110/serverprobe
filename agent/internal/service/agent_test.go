package service

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/huang110/serverprobe/agent/internal/client"
	"github.com/huang110/serverprobe/agent/internal/config"
	pb "github.com/huang110/serverprobe/agent/proto"
)

// agentTestServer 用于 Agent 生命周期测试的本地服务端
type agentTestServer struct {
	pb.UnimplementedProbeServiceServer

	mu                sync.Mutex
	registerCount     int
	lastRegister      *pb.RegisterRequest
	registerResponse  *pb.RegisterResponse
	heartbeatCount    int
	lastHeartbeat     *pb.HeartbeatRequest
	heartbeatResponse *pb.HeartbeatResponse
}

func (s *agentTestServer) Register(_ context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.registerCount++
	s.lastRegister = req
	if s.registerResponse == nil {
		return &pb.RegisterResponse{Success: false, Message: "registration not configured"}, nil
	}
	return s.registerResponse, nil
}

func (s *agentTestServer) Heartbeat(_ context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.heartbeatCount++
	s.lastHeartbeat = req
	if s.heartbeatResponse == nil {
		return &pb.HeartbeatResponse{Success: true}, nil
	}
	return s.heartbeatResponse, nil
}

func (s *agentTestServer) lastHeartbeatRequest() *pb.HeartbeatRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastHeartbeat
}

func (s *agentTestServer) heartbeats() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.heartbeatCount
}

func (s *agentTestServer) registrations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.registerCount
}

// startAgentTestServer 启动本地 gRPC 服务并返回监听地址
func startAgentTestServer(t *testing.T, fake *agentTestServer) string {
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

	return listener.Addr().String()
}

// waitForHeartbeat 等待服务端收到至少一次心跳
func waitForHeartbeat(t *testing.T, fake *agentTestServer, timeout time.Duration) *pb.HeartbeatRequest {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if request := fake.lastHeartbeatRequest(); request != nil {
			return request
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for heartbeat after %s", timeout)
	return nil
}

// TestAgentLifecycle 验证已注册 Agent 的启动、健康检查、心跳和停止流程
func TestAgentLifecycle(t *testing.T) {
	fake := &agentTestServer{
		heartbeatResponse: &pb.HeartbeatResponse{Success: true},
	}
	address := startAgentTestServer(t, fake)
	_, cfg, loader := newHeartbeatTestConfig(t, address)

	agent, err := NewAgent(cfg, loader)
	if err != nil {
		t.Fatalf("NewAgent() returned error: %v", err)
	}
	defer agent.Stop()

	if err := agent.HealthCheck(); err == nil {
		t.Error("HealthCheck() should fail before Start()")
	}

	if err := agent.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}
	if err := agent.HealthCheck(); err != nil {
		t.Errorf("HealthCheck() returned error after Start(): %v", err)
	}
	if err := agent.Start(); err == nil {
		t.Error("second Start() should return an error")
	}

	request := waitForHeartbeat(t, fake, 6*time.Second)
	if request.AgentId != cfg.Agent.ID {
		t.Errorf("heartbeat AgentId = %q, want %q", request.AgentId, cfg.Agent.ID)
	}
	expectedSignature := client.GenerateSignature(request.AgentId, request.Timestamp, cfg.Agent.Token)
	if request.Signature != expectedSignature {
		t.Errorf("heartbeat signature is invalid")
	}
	if request.Cpu == nil || request.Memory == nil || request.System == nil {
		t.Error("heartbeat should contain CPU, memory and system metrics")
	}
	if request.AgentStatus == nil {
		t.Error("heartbeat should contain agent status")
	}
	if fake.registrations() != 0 {
		t.Errorf("already registered agent should not call Register, got %d calls", fake.registrations())
	}

	agent.Stop()
	if err := agent.HealthCheck(); err == nil {
		t.Error("HealthCheck() should fail after Stop()")
	}

	// 重复 Stop 和停止后 Start 都应安全处理
	agent.Stop()
	if err := agent.Start(); err == nil {
		t.Error("Start() after Stop() should return an error")
	}
}

// TestAgentStartRegisters 验证 Agent 首次启动时完成注册并应用服务端心跳间隔
func TestAgentStartRegisters(t *testing.T) {
	newExpiresAt := time.Now().Add(24 * time.Hour).Unix()
	fake := &agentTestServer{
		registerResponse: &pb.RegisterResponse{
			Success:           true,
			AgentId:           "agent-registered",
			Token:             "token-registered",
			TokenExpiresAt:    newExpiresAt,
			HeartbeatInterval: 15,
		},
		heartbeatResponse: &pb.HeartbeatResponse{Success: true},
	}
	address := startAgentTestServer(t, fake)
	configPath, cfg, loader := newHeartbeatTestConfig(t, address)

	// 清除预置凭据，模拟首次注册
	cfg.Agent.ID = ""
	cfg.Agent.Token = ""
	cfg.Agent.TokenExpiresAt = 0

	agent, err := NewAgent(cfg, loader)
	if err != nil {
		t.Fatalf("NewAgent() returned error: %v", err)
	}
	defer agent.Stop()

	// 避免测试依赖外部公网 IP 查询
	agent.register.publicIPLookup = func() string { return "203.0.113.10" }
	agent.register.internalIPLookup = func() string { return "10.0.0.5" }

	if err := agent.Start(); err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	if fake.registrations() != 1 {
		t.Errorf("Register() calls = %d, want 1", fake.registrations())
	}
	if cfg.Agent.ID != "agent-registered" || cfg.Agent.Token != "token-registered" {
		t.Errorf("agent credentials not updated after registration: %+v", cfg.Agent)
	}
	if got := agent.heartbeat.currentInterval(); got != 15*time.Second {
		t.Errorf("heartbeat interval = %s, want 15s", got)
	}

	request := waitForHeartbeat(t, fake, 6*time.Second)
	expectedSignature := client.GenerateSignature(request.AgentId, request.Timestamp, "token-registered")
	if request.AgentId != "agent-registered" || request.Signature != expectedSignature {
		t.Errorf("heartbeat did not use registered credentials: agent=%q", request.AgentId)
	}

	saved := loadSavedConfig(t, configPath)
	if saved.Agent.ID != "agent-registered" || saved.Agent.Token != "token-registered" {
		t.Errorf("registration was not persisted: %+v", saved.Agent)
	}
	if saved.Monitor.HeartbeatInterval != 15 {
		t.Errorf("heartbeat interval not persisted: %d", saved.Monitor.HeartbeatInterval)
	}

	if err := agent.HealthCheck(); err != nil {
		t.Errorf("HealthCheck() returned error: %v", err)
	}
}

// TestNewAgentValidation 验证构造参数和配置校验，不建立实际连接
func TestNewAgentValidation(t *testing.T) {
	if _, err := NewAgent(nil, nil); err == nil {
		t.Error("nil config should return an error")
	}

	if _, err := NewAgent(config.DefaultConfig(), nil); err == nil {
		t.Error("nil loader should return an error")
	}

	invalidAddress := config.DefaultConfig()
	invalidAddress.Server.Address = ""
	invalidAddress.Server.RegisterSecret = "secret"
	if _, err := NewAgent(invalidAddress, config.NewLoader("")); err == nil {
		t.Error("empty server address should return an error")
	}

	missingSecret := config.DefaultConfig()
	missingSecret.Server.Address = "127.0.0.1:50051"
	missingSecret.Server.RegisterSecret = ""
	missingSecret.Agent.ID = ""
	missingSecret.Agent.Token = ""
	if _, err := NewAgent(missingSecret, config.NewLoader("")); err == nil {
		t.Error("missing register_secret for first registration should return an error")
	}
}
