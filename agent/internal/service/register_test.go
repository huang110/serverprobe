package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gopkg.in/yaml.v3"

	"github.com/huang110/serverprobe/agent/internal/client"
	"github.com/huang110/serverprobe/agent/internal/config"
	pb "github.com/huang110/serverprobe/agent/proto"
)

// registerTestServer 用于验证注册请求的本地服务端
type registerTestServer struct {
	pb.UnimplementedProbeServiceServer

	mu       sync.Mutex
	request  *pb.RegisterRequest
	response *pb.RegisterResponse
	err      error
}

func (s *registerTestServer) Register(_ context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.request = req
	return s.response, s.err
}

func (s *registerTestServer) lastRequest() *pb.RegisterRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.request
}

// startRegisterTestServer 启动本地 gRPC 服务并返回真实客户端
func startRegisterTestServer(t *testing.T, fake *registerTestServer) *client.GRPCClient {
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
		client.WithRequestTimeout(3*time.Second),
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

// newRegisterTestConfig 创建临时配置文件并加载配置
func newRegisterTestConfig(t *testing.T, address, registerSecret string) (string, *config.Config, *config.Loader) {
	t.Helper()

	configPath := filepath.Join(t.TempDir(), "agent.yaml")
	content := fmt.Sprintf(`agent:
  id: ""
  token: ""
  token_expires_at: 0
  lite_mode: true
server:
  address: %q
  register_secret: %q
  tls_mode: "none"
monitor:
  heartbeat_interval: 30
labels:
  env: test
  role: agent
log:
  level: "error"
  file: ""
`, address, registerSecret)

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

// TestRegisterSuccess 验证注册请求内容、内存配置和持久化结果
func TestRegisterSuccess(t *testing.T) {
	expiresAt := time.Now().Add(24 * time.Hour).Unix()
	fake := &registerTestServer{
		response: &pb.RegisterResponse{
			Success:           true,
			Message:           "ok",
			AgentId:           "agent-123",
			Token:             "token-123",
			HeartbeatInterval: 15,
			TokenExpiresAt:    expiresAt,
		},
	}
	grpcClient := startRegisterTestServer(t, fake)

	configPath, cfg, loader := newRegisterTestConfig(t, "127.0.0.1:50051", "test-secret")
	svc := NewRegisterService(grpcClient, cfg, loader)
	svc.publicIPLookup = func() string { return "203.0.113.10" }
	svc.internalIPLookup = func() string { return "10.0.0.5" }

	if err := svc.Register(); err != nil {
		t.Fatalf("Register() returned error: %v", err)
	}

	request := fake.lastRequest()
	if request == nil {
		t.Fatal("server did not receive registration request")
	}
	if request.RegisterSecret != "test-secret" {
		t.Errorf("RegisterSecret = %q, want %q", request.RegisterSecret, "test-secret")
	}
	if request.Hostname == "" {
		t.Error("Hostname should not be empty")
	}
	if request.Ip != "203.0.113.10" {
		t.Errorf("Ip = %q, want %q", request.Ip, "203.0.113.10")
	}
	if request.InternalIp != "10.0.0.5" {
		t.Errorf("InternalIp = %q, want %q", request.InternalIp, "10.0.0.5")
	}
	if request.Os != runtime.GOOS {
		t.Errorf("Os = %q, want %q", request.Os, runtime.GOOS)
	}
	if request.OsVersion == "" {
		t.Error("OsVersion should not be empty")
	}
	if request.CpuCores <= 0 {
		t.Errorf("CpuCores should be positive, got %d", request.CpuCores)
	}
	if request.MemoryTotal <= 0 {
		t.Errorf("MemoryTotal should be positive, got %d", request.MemoryTotal)
	}
	if request.DiskTotal < 0 {
		t.Errorf("DiskTotal should not be negative, got %d", request.DiskTotal)
	}
	if request.Arch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q", request.Arch, runtime.GOARCH)
	}
	if request.AgentVersion != config.Version {
		t.Errorf("AgentVersion = %q, want %q", request.AgentVersion, config.Version)
	}
	if request.Labels["env"] != "test" || request.Labels["role"] != "agent" {
		t.Errorf("unexpected labels: %+v", request.Labels)
	}
	if !request.LiteMode {
		t.Error("LiteMode should be true")
	}

	if cfg.Agent.ID != "agent-123" || cfg.Agent.Token != "token-123" || cfg.Agent.TokenExpiresAt != expiresAt {
		t.Errorf("in-memory agent config not updated: %+v", cfg.Agent)
	}
	if cfg.Monitor.HeartbeatInterval != 15 {
		t.Errorf("heartbeat interval = %d, want 15", cfg.Monitor.HeartbeatInterval)
	}

	saved := loadSavedConfig(t, configPath)
	if saved.Agent.ID != "agent-123" || saved.Agent.Token != "token-123" || saved.Agent.TokenExpiresAt != expiresAt {
		t.Errorf("saved agent config is incorrect: %+v", saved.Agent)
	}
	if saved.Monitor.HeartbeatInterval != 15 {
		t.Errorf("saved heartbeat interval = %d, want 15", saved.Monitor.HeartbeatInterval)
	}
}

// TestRegisterRejected 验证服务端业务拒绝时不会保存凭据
func TestRegisterRejected(t *testing.T) {
	fake := &registerTestServer{
		response: &pb.RegisterResponse{
			Success: false,
			Message: "invalid register secret",
		},
	}
	grpcClient := startRegisterTestServer(t, fake)

	configPath, cfg, loader := newRegisterTestConfig(t, "127.0.0.1:50051", "wrong-secret")
	svc := NewRegisterService(grpcClient, cfg, loader)
	svc.publicIPLookup = func() string { return "203.0.113.10" }
	svc.internalIPLookup = func() string { return "10.0.0.5" }

	err := svc.Register()
	if err == nil {
		t.Fatal("Register() should return an error when server rejects the request")
	}
	if !strings.Contains(err.Error(), "registration rejected") {
		t.Errorf("unexpected error: %v", err)
	}
	if !errors.Is(err, ErrRegistrationRejected) {
		t.Errorf("error should wrap ErrRegistrationRejected: %v", err)
	}
	if cfg.Agent.ID != "" || cfg.Agent.Token != "" {
		t.Errorf("agent credentials should not be set after rejection: %+v", cfg.Agent)
	}

	saved := loadSavedConfig(t, configPath)
	if saved.Agent.ID != "" || saved.Agent.Token != "" {
		t.Errorf("saved config should not contain credentials after rejection: %+v", saved.Agent)
	}
}

// TestRegisterRPCError 验证网络错误与业务拒绝错误被区分
func TestRegisterRPCError(t *testing.T) {
	fake := &registerTestServer{
		err: status.Error(codes.Unavailable, "connection lost"),
	}
	grpcClient := startRegisterTestServer(t, fake)

	_, cfg, loader := newRegisterTestConfig(t, "127.0.0.1:50051", "test-secret")
	svc := NewRegisterService(grpcClient, cfg, loader)
	svc.publicIPLookup = func() string { return "203.0.113.10" }
	svc.internalIPLookup = func() string { return "10.0.0.5" }

	err := svc.Register()
	if err == nil {
		t.Fatal("Register() should return an error on RPC failure")
	}
	if !strings.Contains(err.Error(), "registration failed") {
		t.Errorf("unexpected error: %v", err)
	}
	if strings.Contains(err.Error(), "registration rejected") {
		t.Errorf("RPC error should not be reported as a business rejection: %v", err)
	}
}

// TestRegisterAlreadyRegistered 验证已注册且 Token 未过期时跳过注册
func TestRegisterAlreadyRegistered(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agent.ID = "existing-agent"
	cfg.Agent.Token = "existing-token"
	cfg.Agent.TokenExpiresAt = time.Now().Add(time.Hour).Unix()

	svc := NewRegisterService(nil, cfg, config.NewLoader(""))

	if err := svc.Register(); err != nil {
		t.Errorf("Register() returned error for registered agent: %v", err)
	}
}

// TestRegisterMissingSecret 验证首次注册缺少 register_secret 时立即失败
func TestRegisterMissingSecret(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Server.RegisterSecret = ""

	svc := NewRegisterService(nil, cfg, config.NewLoader(""))

	err := svc.Register()
	if err == nil {
		t.Fatal("Register() should return an error without register_secret")
	}
	if !strings.Contains(err.Error(), "register_secret") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestGetHostnameEnvOverride 验证 Docker 部署时可通过环境变量覆盖主机名
func TestGetHostnameEnvOverride(t *testing.T) {
	t.Setenv("AGENT_HOSTNAME", "docker-host-01")

	hostname, err := getHostname()
	if err != nil {
		t.Fatalf("getHostname() failed: %v", err)
	}
	if hostname != "docker-host-01" {
		t.Errorf("hostname = %q, want docker-host-01", hostname)
	}
}

// TestFormatOSVersion 验证系统版本拼接逻辑
func TestFormatOSVersion(t *testing.T) {
	tests := []struct {
		platform string
		version  string
		want     string
	}{
		{"Ubuntu", "22.04", "Ubuntu 22.04"},
		{"Ubuntu", "", "Ubuntu"},
		{"", "22.04", "22.04"},
		{"", "", runtime.GOOS},
	}

	for _, test := range tests {
		if got := formatOSVersion(test.platform, test.version); got != test.want {
			t.Errorf("formatOSVersion(%q, %q) = %q, want %q", test.platform, test.version, got, test.want)
		}
	}
}

// loadSavedConfig 读取并解析持久化后的配置文件
func loadSavedConfig(t *testing.T, path string) *config.Config {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read saved config: %v", err)
	}

	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse saved config: %v", err)
	}
	return &cfg
}
