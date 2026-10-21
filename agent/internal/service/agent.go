package service

import (
	"errors"
	"fmt"
	"sync"

	"github.com/huang110/serverprobe/agent/internal/client"
	"github.com/huang110/serverprobe/agent/internal/config"
	"github.com/huang110/serverprobe/agent/pkg/logger"
)

// Agent 聚合 gRPC 客户端、注册服务和心跳服务，负责整体生命周期管理。
type Agent struct {
	config     *config.Config
	loader     *config.Loader
	grpcClient *client.GRPCClient
	register   *RegisterService
	heartbeat  *HeartbeatService

	mu      sync.Mutex
	started bool
	stopped bool
}

// NewAgent 创建 Agent 并建立到 Server 的 gRPC 连接。
func NewAgent(cfg *config.Config, loader *config.Loader) (*Agent, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if loader == nil {
		return nil, errors.New("config loader is required")
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	grpcClient, err := client.NewGRPCClient(&cfg.Server)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	return &Agent{
		config:     cfg,
		loader:     loader,
		grpcClient: grpcClient,
		register:   NewRegisterService(grpcClient, cfg, loader),
		heartbeat:  NewHeartbeatService(grpcClient, cfg, loader),
	}, nil
}

// Start 依次执行注册并启动心跳服务。
// 启动失败时 Agent 保持未启动状态，但 gRPC 连接不会关闭，允许调用方修复配置后重试。
func (a *Agent) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return errors.New("agent has been stopped and cannot be restarted")
	}
	if a.started {
		return errors.New("agent already started")
	}

	if a.register == nil {
		return errors.New("register service is not initialized")
	}
	if a.heartbeat == nil {
		return errors.New("heartbeat service is not initialized")
	}

	// 1. 注册（已注册且 Token 未过期时会直接返回）
	if err := a.register.Register(); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	// 2. 启动心跳服务
	if err := a.heartbeat.Start(); err != nil {
		return fmt.Errorf("failed to start heartbeat service: %w", err)
	}

	a.started = true
	logger.Info("Agent started successfully")
	return nil
}

// Stop 停止心跳服务并关闭 gRPC 连接。
// 可安全重复调用；未启动或已停止时不会发生任何操作。
func (a *Agent) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return
	}
	a.stopped = true

	logger.Info("Stopping agent...")

	// 1. 停止心跳，等待正在执行的心跳完成
	if a.heartbeat != nil {
		a.heartbeat.Stop()
	}

	// 2. 关闭 gRPC 连接
	if a.grpcClient != nil {
		if err := a.grpcClient.Close(); err != nil {
			logger.Errorf("Failed to close gRPC connection: %v", err)
		}
	}

	logger.Info("Agent stopped")
}

// HealthCheck 检查 Agent 是否处于健康运行状态。
// 健康条件：已启动、未停止、心跳循环运行中、已完成注册。
func (a *Agent) HealthCheck() error {
	a.mu.Lock()
	started := a.started
	stopped := a.stopped
	heartbeat := a.heartbeat
	cfg := a.config
	a.mu.Unlock()

	if !started || stopped {
		return errors.New("agent is not running")
	}
	if heartbeat == nil || !heartbeat.IsRunning() {
		return errors.New("heartbeat service is not running")
	}
	// Agent.ID 在注册完成后不再修改，可安全读取；
	// Token 会在心跳过程中轮换，不在健康检查中读取以避免数据竞争。
	if cfg == nil || cfg.Agent.ID == "" {
		return errors.New("agent is not registered")
	}

	return nil
}
