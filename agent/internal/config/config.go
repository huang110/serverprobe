package config

import (
	"fmt"
	"os"
	"time"
)

const Version = "1.0.1"

// Config Agent 配置结构
type Config struct {
	Agent     AgentConfig       `yaml:"agent"`
	Server    ServerConfig      `yaml:"server"`
	Monitor   MonitorConfig     `yaml:"monitor"`
	IPQuality IPQualityConfig   `yaml:"ip_quality"`
	Log       LogConfig         `yaml:"log"`
	Labels    map[string]string `yaml:"labels"`
}

// AgentConfig Agent 基础配置
type AgentConfig struct {
	ID             string `yaml:"id"`
	Token          string `yaml:"token"`
	TokenExpiresAt int64  `yaml:"token_expires_at"`
	LiteMode       bool   `yaml:"lite_mode"`
}

// ServerConfig 服务端配置
type ServerConfig struct {
	Address        string `yaml:"address"`
	RegisterSecret string `yaml:"register_secret"`
	TLSMode        string `yaml:"tls_mode"` // simple/mutual
	CACert         string `yaml:"ca_cert"`
	ClientCert     string `yaml:"client_cert"`
	ClientKey      string `yaml:"client_key"`
}

// MonitorConfig 监控配置
type MonitorConfig struct {
	HeartbeatInterval int      `yaml:"heartbeat_interval"` // 秒
	CollectInterval   int      `yaml:"collect_interval"`   // 秒
	NetworkInterfaces []string `yaml:"network_interfaces"`
	DiskMounts        []string `yaml:"disk_mounts"`
	ExcludeFSTypes    []string `yaml:"exclude_fs_types"`
}

// IPQualityConfig IP 质量检测配置
type IPQualityConfig struct {
	Enabled          bool            `yaml:"enabled"`
	CheckInterval    int             `yaml:"check_interval"` // 秒
	InitialFastCheck bool            `yaml:"initial_fast_check"`
	LatencyTargets   []LatencyTarget `yaml:"latency_targets"`
}

// LatencyTarget 延迟测试目标
type LatencyTarget struct {
	Name    string `yaml:"name"`
	Address string `yaml:"address"`
}

// LogConfig 日志配置
type LogConfig struct {
	Level   string `yaml:"level"` // debug/info/warn/error
	File    string `yaml:"file"`
	MaxSize int    `yaml:"max_size"` // MB
	MaxDays int    `yaml:"max_days"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Agent: AgentConfig{
			LiteMode: false,
		},
		Server: ServerConfig{
			Address:        "https://localhost:50051",
			RegisterSecret: os.Getenv("SERVERPROBE_REGISTER_SECRET"),
			TLSMode:        "simple",
		},
		Monitor: MonitorConfig{
			HeartbeatInterval: 30,
			CollectInterval:   30,
			NetworkInterfaces: []string{"eth*", "ens*", "en*"},
			DiskMounts:        []string{"/", "/home", "/data"},
			ExcludeFSTypes:    []string{"tmpfs", "devtmpfs", "squashfs"},
		},
		IPQuality: IPQualityConfig{
			Enabled:          true,
			CheckInterval:    15,
			InitialFastCheck: true,
			LatencyTargets: []LatencyTarget{
				{Name: "电信", Address: "202.102.192.68:53"},
				{Name: "联通", Address: "210.22.70.3:53"},
				{Name: "移动", Address: "211.136.192.6:53"},
				{Name: "阿里DNS", Address: "223.5.5.5:443"},
				{Name: "Cloudflare", Address: "1.1.1.1:443"},
			},
		},
		Log: LogConfig{
			Level:   "info",
			File:    "/var/log/serverprobe/agent.log",
			MaxSize: 100,
			MaxDays: 7,
		},
		Labels: make(map[string]string),
	}
}

// Validate 验证配置
func (c *Config) Validate() error {
	if c.Server.Address == "" {
		return fmt.Errorf("server.address is required")
	}

	if c.Agent.ID == "" && c.Server.RegisterSecret == "" {
		return fmt.Errorf("server.register_secret is required for first registration")
	}

	if c.Monitor.HeartbeatInterval < 1 {
		return fmt.Errorf("monitor.heartbeat_interval must be >= 1")
	}

	return nil
}

// IsRegistered 检查是否已注册
func (c *Config) IsRegistered() bool {
	return c.Agent.ID != "" && c.Agent.Token != ""
}

// IsTokenExpired 检查 Token 是否过期
func (c *Config) IsTokenExpired() bool {
	if c.Agent.TokenExpiresAt == 0 {
		return false
	}
	return time.Now().Unix() >= c.Agent.TokenExpiresAt
}

// GetHeartbeatInterval 获取心跳间隔时长
func (c *Config) GetHeartbeatInterval() time.Duration {
	return time.Duration(c.Monitor.HeartbeatInterval) * time.Second
}

// GetCollectInterval 获取采集间隔时长
func (c *Config) GetCollectInterval() time.Duration {
	interval := c.Monitor.CollectInterval
	if interval == 0 {
		interval = c.Monitor.HeartbeatInterval
	}
	return time.Duration(interval) * time.Second
}

// GetIPQualityInterval 获取 IP 质量检测间隔
func (c *Config) GetIPQualityInterval() time.Duration {
	if c == nil || c.IPQuality.CheckInterval <= 0 {
		return 15 * time.Second
	}
	return time.Duration(c.IPQuality.CheckInterval) * time.Second
}
