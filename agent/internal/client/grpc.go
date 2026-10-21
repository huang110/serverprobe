package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/huang110/serverprobe/agent/internal/config"
	"github.com/huang110/serverprobe/agent/pkg/logger"
	pb "github.com/huang110/serverprobe/agent/proto"
)

const (
	// defaultConnectTimeout 单次连接尝试的超时时间
	defaultConnectTimeout = 10 * time.Second

	// defaultRequestTimeout 单次 RPC 调用的超时时间
	defaultRequestTimeout = 10 * time.Second

	// defaultMaxConnectAttempts 建立连接的最大尝试次数
	defaultMaxConnectAttempts = 5

	// initialReconnectBackoff 首次重试等待时间
	initialReconnectBackoff = time.Second

	// maxReconnectBackoff 最大重试等待时间
	maxReconnectBackoff = 60 * time.Second
)

// GRPCClient gRPC 客户端。
// 内部共享同一个 grpc.ClientConn，所有 RPC 复用该连接。
type GRPCClient struct {
	conn           *grpc.ClientConn
	client         pb.ProbeServiceClient
	config         *config.ServerConfig
	requestTimeout time.Duration

	closeOnce sync.Once
	closeErr  error
}

// GRPCClientOptions gRPC 客户端可调参数
type GRPCClientOptions struct {
	ConnectTimeout     time.Duration
	RequestTimeout     time.Duration
	MaxConnectAttempts int
	InitialBackoff     time.Duration
	MaxBackoff         time.Duration
}

// GRPCClientOption 修改 gRPC 客户端参数
type GRPCClientOption func(*GRPCClientOptions)

// WithConnectTimeout 设置单次连接尝试的超时时间
func WithConnectTimeout(timeout time.Duration) GRPCClientOption {
	return func(o *GRPCClientOptions) {
		o.ConnectTimeout = timeout
	}
}

// WithRequestTimeout 设置单次 RPC 调用的超时时间
func WithRequestTimeout(timeout time.Duration) GRPCClientOption {
	return func(o *GRPCClientOptions) {
		o.RequestTimeout = timeout
	}
}

// WithMaxConnectAttempts 设置建立连接的最大尝试次数
func WithMaxConnectAttempts(attempts int) GRPCClientOption {
	return func(o *GRPCClientOptions) {
		o.MaxConnectAttempts = attempts
	}
}

// WithReconnectBackoff 设置重试退避的初始值和最大值
func WithReconnectBackoff(initial, maximum time.Duration) GRPCClientOption {
	return func(o *GRPCClientOptions) {
		o.InitialBackoff = initial
		o.MaxBackoff = maximum
	}
}

// NewGRPCClient 建立到 Server 的 gRPC 连接。
// 连接失败时按指数退避重试（1s、2s、4s...，最大 60s），达到最大次数后返回错误。
// gRPC ClientConn 建立后会由 gRPC 自动维护重连，无需额外处理。
func NewGRPCClient(cfg *config.ServerConfig, opts ...GRPCClientOption) (*GRPCClient, error) {
	if cfg == nil {
		return nil, errors.New("server config is required")
	}

	address := normalizeAddress(cfg.Address)
	if address == "" {
		return nil, errors.New("server address is required")
	}

	options := defaultGRPCClientOptions()
	for _, opt := range opts {
		opt(&options)
	}
	options = sanitizeGRPCClientOptions(options)

	transportCredentials, err := buildTransportCredentials(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to configure transport credentials: %w", err)
	}

	dialOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(transportCredentials),
		grpc.WithBlock(),
	}

	var lastErr error
	backoff := options.InitialBackoff
	for attempt := 1; attempt <= options.MaxConnectAttempts; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), options.ConnectTimeout)
		conn, err := grpc.DialContext(ctx, address, dialOptions...)
		cancel()

		if err == nil {
			logger.Infof("Connected to server %s (tls_mode: %s)", address, normalizeTLSMode(cfg.TLSMode))
			return &GRPCClient{
				conn:           conn,
				client:         pb.NewProbeServiceClient(conn),
				config:         cfg,
				requestTimeout: options.RequestTimeout,
			}, nil
		}

		lastErr = err
		logger.Warnf("Failed to connect to server %s (attempt %d/%d): %v",
			address, attempt, options.MaxConnectAttempts, err)

		if attempt < options.MaxConnectAttempts {
			time.Sleep(backoff)
			backoff = nextBackoff(backoff, options.MaxBackoff)
		}
	}

	return nil, fmt.Errorf("failed to connect to %s after %d attempts: %w",
		address, options.MaxConnectAttempts, lastErr)
}

// Register 调用注册接口
func (c *GRPCClient) Register(req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	ctx, cancel := c.newRequestContext()
	defer cancel()

	resp, err := c.client.Register(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("register RPC failed: %w", err)
	}
	return resp, nil
}

// Heartbeat 调用心跳接口
func (c *GRPCClient) Heartbeat(req *pb.HeartbeatRequest) (*pb.HeartbeatResponse, error) {
	ctx, cancel := c.newRequestContext()
	defer cancel()

	resp, err := c.client.Heartbeat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("heartbeat RPC failed: %w", err)
	}
	return resp, nil
}

// GetConfig 获取服务端配置
func (c *GRPCClient) GetConfig(req *pb.GetConfigRequest) (*pb.GetConfigResponse, error) {
	ctx, cancel := c.newRequestContext()
	defer cancel()

	resp, err := c.client.GetConfig(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get config RPC failed: %w", err)
	}
	return resp, nil
}

// RotateToken 轮换 Agent Token
func (c *GRPCClient) RotateToken(req *pb.RotateTokenRequest) (*pb.RotateTokenResponse, error) {
	ctx, cancel := c.newRequestContext()
	defer cancel()

	resp, err := c.client.RotateToken(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("rotate token RPC failed: %w", err)
	}
	return resp, nil
}

// Close 关闭 gRPC 连接，可安全重复调用
func (c *GRPCClient) Close() error {
	c.closeOnce.Do(func() {
		if c.conn != nil {
			c.closeErr = c.conn.Close()
		}
	})
	return c.closeErr
}

// newRequestContext 为单次 RPC 创建带超时的上下文
func (c *GRPCClient) newRequestContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), c.requestTimeout)
}

// defaultGRPCClientOptions 返回默认客户端参数
func defaultGRPCClientOptions() GRPCClientOptions {
	return GRPCClientOptions{
		ConnectTimeout:     defaultConnectTimeout,
		RequestTimeout:     defaultRequestTimeout,
		MaxConnectAttempts: defaultMaxConnectAttempts,
		InitialBackoff:     initialReconnectBackoff,
		MaxBackoff:         maxReconnectBackoff,
	}
}

// sanitizeGRPCClientOptions 修正非法参数，避免无限重试或零超时
func sanitizeGRPCClientOptions(options GRPCClientOptions) GRPCClientOptions {
	if options.ConnectTimeout <= 0 {
		options.ConnectTimeout = defaultConnectTimeout
	}
	if options.RequestTimeout <= 0 {
		options.RequestTimeout = defaultRequestTimeout
	}
	if options.MaxConnectAttempts <= 0 {
		options.MaxConnectAttempts = defaultMaxConnectAttempts
	}
	if options.InitialBackoff <= 0 {
		options.InitialBackoff = initialReconnectBackoff
	}
	if options.MaxBackoff <= 0 {
		options.MaxBackoff = maxReconnectBackoff
	}
	if options.InitialBackoff > options.MaxBackoff {
		options.InitialBackoff = options.MaxBackoff
	}
	return options
}

// nextBackoff 计算下一次指数退避时间，超过上限时保持在上限
func nextBackoff(current, maximum time.Duration) time.Duration {
	if current <= 0 {
		return initialReconnectBackoff
	}
	if current >= maximum {
		return maximum
	}
	if current > maximum/2 {
		return maximum
	}
	return current * 2
}

// buildTransportCredentials 根据 TLS 模式构造传输凭据
func buildTransportCredentials(cfg *config.ServerConfig) (credentials.TransportCredentials, error) {
	switch normalizeTLSMode(cfg.TLSMode) {
	case "simple":
		return buildSimpleTLS(cfg)
	case "mutual":
		return buildMutualTLS(cfg)
	case "none":
		logger.Warn("TLS is disabled, this should only be used in development environments")
		return insecure.NewCredentials(), nil
	default:
		return nil, fmt.Errorf("unsupported tls_mode %q (expected simple, mutual or none)", cfg.TLSMode)
	}
}

// buildSimpleTLS 构造单向 TLS 凭据（客户端验证服务端证书）
func buildSimpleTLS(cfg *config.ServerConfig) (credentials.TransportCredentials, error) {
	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.CACert != "" {
		certPool, err := loadCertPool(cfg.CACert)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = certPool
	}

	return credentials.NewTLS(tlsConfig), nil
}

// buildMutualTLS 构造双向 TLS 凭据（客户端和服务端互相验证证书）
func buildMutualTLS(cfg *config.ServerConfig) (credentials.TransportCredentials, error) {
	if cfg.ClientCert == "" || cfg.ClientKey == "" {
		return nil, errors.New("client_cert and client_key are required for mutual TLS")
	}

	certificate, err := tls.LoadX509KeyPair(cfg.ClientCert, cfg.ClientKey)
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificate: %w", err)
	}

	tlsConfig := &tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{certificate},
	}

	if cfg.CACert != "" {
		certPool, err := loadCertPool(cfg.CACert)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = certPool
	}

	return credentials.NewTLS(tlsConfig), nil
}

// loadCertPool 从 PEM 文件加载 CA 证书池
func loadCertPool(caCertFile string) (*x509.CertPool, error) {
	pemData, err := os.ReadFile(caCertFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate %s: %w", caCertFile, err)
	}

	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(pemData) {
		return nil, fmt.Errorf("no valid certificates found in %s", caCertFile)
	}
	return certPool, nil
}

// normalizeTLSMode 规范化 TLS 模式，空值默认使用 simple
func normalizeTLSMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		return "simple"
	}
	if mode == "insecure" {
		return "none"
	}
	return mode
}

// normalizeAddress 去除 Server 地址中的协议前缀和末尾斜杠。
// gRPC 的 Dial target 需要 host:port 形式，不能直接使用 https:// URL。
func normalizeAddress(address string) string {
	address = strings.TrimSpace(address)
	if address == "" {
		return ""
	}

	if strings.Contains(address, "://") {
		if parsed, err := url.Parse(address); err == nil && parsed.Host != "" {
			return parsed.Host
		}
	}

	return strings.TrimSuffix(address, "/")
}
