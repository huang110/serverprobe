package client

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/huang110/serverprobe/agent/internal/config"
	pb "github.com/huang110/serverprobe/agent/proto"
)

// testProbeServer 用于验证客户端 RPC 调用的本地服务端
type testProbeServer struct {
	pb.UnimplementedProbeServiceServer
}

func (s *testProbeServer) Register(_ context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	return &pb.RegisterResponse{
		Success:           true,
		Message:           "ok",
		AgentId:           "agent-" + req.Hostname,
		Token:             "token-1",
		HeartbeatInterval: 30,
		TokenExpiresAt:    1700000000,
	}, nil
}

func (s *testProbeServer) Heartbeat(_ context.Context, req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	return &pb.HeartbeatResponse{
		Success:               true,
		NewToken:              "new-" + req.AgentId,
		NewTokenExpiresAt:     1700000000,
		NextHeartbeatInterval: 15,
	}, nil
}

func (s *testProbeServer) GetConfig(_ context.Context, req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	return &pb.GetConfigResponse{
		Success:           true,
		HeartbeatInterval: 20,
		Labels:            map[string]string{"agent": req.AgentId},
	}, nil
}

func (s *testProbeServer) RotateToken(_ context.Context, req *pb.RotateTokenRequest) (*pb.RotateTokenResponse, error) {
	return &pb.RotateTokenResponse{
		Success:   true,
		NewToken:  "rotated-" + req.AgentId,
		ExpiresAt: 1700000000,
	}, nil
}

// TestGRPCClientRPCs 启动本地 gRPC 服务，验证 4 个 RPC 和连接关闭
func TestGRPCClientRPCs(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}

	server := grpc.NewServer()
	pb.RegisterProbeServiceServer(server, &testProbeServer{})
	go func() {
		if serveErr := server.Serve(listener); serveErr != nil {
			t.Logf("test gRPC server stopped: %v", serveErr)
		}
	}()
	defer server.Stop()

	client, err := NewGRPCClient(
		&config.ServerConfig{
			Address: listener.Addr().String(),
			TLSMode: "none",
		},
		WithConnectTimeout(3*time.Second),
		WithRequestTimeout(3*time.Second),
		WithMaxConnectAttempts(1),
	)
	if err != nil {
		t.Fatalf("NewGRPCClient() returned error: %v", err)
	}
	defer func() {
		if closeErr := client.Close(); closeErr != nil {
			t.Errorf("Close() returned error: %v", closeErr)
		}
	}()

	t.Run("Register", func(t *testing.T) {
		resp, err := client.Register(&pb.RegisterRequest{Hostname: "node-1"})
		if err != nil {
			t.Fatalf("Register() returned error: %v", err)
		}
		if !resp.Success || resp.AgentId != "agent-node-1" || resp.Token != "token-1" {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("Heartbeat", func(t *testing.T) {
		resp, err := client.Heartbeat(&pb.HeartbeatRequest{AgentId: "agent-1"})
		if err != nil {
			t.Fatalf("Heartbeat() returned error: %v", err)
		}
		if !resp.Success || resp.NewToken != "new-agent-1" || resp.NextHeartbeatInterval != 15 {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("GetConfig", func(t *testing.T) {
		resp, err := client.GetConfig(&pb.GetConfigRequest{AgentId: "agent-1"})
		if err != nil {
			t.Fatalf("GetConfig() returned error: %v", err)
		}
		if !resp.Success || resp.HeartbeatInterval != 20 || resp.Labels["agent"] != "agent-1" {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("RotateToken", func(t *testing.T) {
		resp, err := client.RotateToken(&pb.RotateTokenRequest{AgentId: "agent-1"})
		if err != nil {
			t.Fatalf("RotateToken() returned error: %v", err)
		}
		if !resp.Success || resp.NewToken != "rotated-agent-1" {
			t.Errorf("unexpected response: %+v", resp)
		}
	})

	t.Run("CloseIsIdempotent", func(t *testing.T) {
		if err := client.Close(); err != nil {
			t.Errorf("first Close() returned error: %v", err)
		}
		if err := client.Close(); err != nil {
			t.Errorf("second Close() returned error: %v", err)
		}
	})
}

// TestNewGRPCClientValidation 验证构造参数校验，不进行实际连接
func TestNewGRPCClientValidation(t *testing.T) {
	if _, err := NewGRPCClient(nil); err == nil {
		t.Error("nil config should return an error")
	}

	if _, err := NewGRPCClient(&config.ServerConfig{Address: "", TLSMode: "none"}); err == nil {
		t.Error("empty address should return an error")
	}

	if _, err := NewGRPCClient(&config.ServerConfig{Address: "127.0.0.1:1", TLSMode: "invalid"}); err == nil {
		t.Error("unsupported tls_mode should return an error")
	}
}

// TestNewGRPCClientRetryFailure 验证连接失败时按配置重试并返回错误
func TestNewGRPCClientRetryFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	address := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("failed to close listener: %v", err)
	}

	_, err = NewGRPCClient(
		&config.ServerConfig{Address: address, TLSMode: "none"},
		WithConnectTimeout(200*time.Millisecond),
		WithMaxConnectAttempts(2),
		WithReconnectBackoff(time.Millisecond, 2*time.Millisecond),
	)
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
	if !strings.Contains(err.Error(), "after 2 attempts") {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestBuildTransportCredentials 验证 TLS 模式构造和证书错误处理
func TestBuildTransportCredentials(t *testing.T) {
	t.Run("insecure", func(t *testing.T) {
		creds, err := buildTransportCredentials(&config.ServerConfig{TLSMode: "none"})
		if err != nil || creds == nil {
			t.Errorf("insecure credentials failed: creds=%v err=%v", creds, err)
		}
	})

	t.Run("simple without custom CA", func(t *testing.T) {
		creds, err := buildTransportCredentials(&config.ServerConfig{TLSMode: "simple"})
		if err != nil || creds == nil {
			t.Errorf("simple TLS credentials failed: creds=%v err=%v", creds, err)
		}
	})

	t.Run("unsupported mode", func(t *testing.T) {
		if _, err := buildTransportCredentials(&config.ServerConfig{TLSMode: "bogus"}); err == nil {
			t.Error("unsupported mode should return an error")
		}
	})

	t.Run("mutual without client cert", func(t *testing.T) {
		if _, err := buildTransportCredentials(&config.ServerConfig{TLSMode: "mutual"}); err == nil {
			t.Error("mutual TLS without client cert should return an error")
		}
	})

	t.Run("missing CA file", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.pem")
		if _, err := buildTransportCredentials(&config.ServerConfig{TLSMode: "simple", CACert: missing}); err == nil {
			t.Error("missing CA file should return an error")
		}
	})

	t.Run("invalid CA content", func(t *testing.T) {
		invalid := filepath.Join(t.TempDir(), "invalid.pem")
		if err := os.WriteFile(invalid, []byte("not a certificate"), 0600); err != nil {
			t.Fatalf("failed to write temp CA file: %v", err)
		}
		if _, err := buildTransportCredentials(&config.ServerConfig{TLSMode: "simple", CACert: invalid}); err == nil {
			t.Error("invalid CA content should return an error")
		}
	})
}

// TestNormalizeAddress 验证 Server 地址规范化
func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		address string
		want    string
	}{
		{"https://monitor.example.com:50051", "monitor.example.com:50051"},
		{"http://127.0.0.1:50051/", "127.0.0.1:50051"},
		{"  monitor.example.com:50051  ", "monitor.example.com:50051"},
		{"[::1]:50051", "[::1]:50051"},
		{"", ""},
	}

	for _, test := range tests {
		if got := normalizeAddress(test.address); got != test.want {
			t.Errorf("normalizeAddress(%q) = %q, want %q", test.address, got, test.want)
		}
	}
}

// TestNormalizeTLSMode 验证 TLS 模式规范化
func TestNormalizeTLSMode(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{"", "simple"},
		{"simple", "simple"},
		{" MUTUAL ", "mutual"},
		{"insecure", "none"},
		{"none", "none"},
	}

	for _, test := range tests {
		if got := normalizeTLSMode(test.mode); got != test.want {
			t.Errorf("normalizeTLSMode(%q) = %q, want %q", test.mode, got, test.want)
		}
	}
}

// TestNextBackoff 验证指数退避和上限保护
func TestNextBackoff(t *testing.T) {
	tests := []struct {
		current time.Duration
		maximum time.Duration
		want    time.Duration
	}{
		{time.Second, maxReconnectBackoff, 2 * time.Second},
		{16 * time.Second, maxReconnectBackoff, 32 * time.Second},
		{40 * time.Second, maxReconnectBackoff, maxReconnectBackoff},
		{maxReconnectBackoff, maxReconnectBackoff, maxReconnectBackoff},
		{0, maxReconnectBackoff, initialReconnectBackoff},
	}

	for _, test := range tests {
		if got := nextBackoff(test.current, test.maximum); got != test.want {
			t.Errorf("nextBackoff(%v, %v) = %v, want %v", test.current, test.maximum, got, test.want)
		}
	}
}
