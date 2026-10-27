package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

const Version = "1.0.1"

const (
	// DefaultHeartbeatInterval 默认心跳间隔（秒）
	DefaultHeartbeatInterval = 5

	// DefaultTokenExpirationDays 默认 Token 有效期（天）
	DefaultTokenExpirationDays = 7

	// DefaultAlertCheckInterval 默认告警检查间隔（秒）
	DefaultAlertCheckInterval = 30

	// DefaultRateLimitPerSecond 默认每秒请求数
	DefaultRateLimitPerSecond = 100
)

// Config Server 配置。
// 同时保留顶层兼容字段（RegisterSecret/HeartbeatInterval 等），
// 以便既有代码和测试无需改动。
type Config struct {
	Server      ServerConfig      `yaml:"server" mapstructure:"server"`
	Database    DatabaseConfig    `yaml:"database" mapstructure:"database"`
	Redis       RedisConfig       `yaml:"redis" mapstructure:"redis"`
	Agent       AgentConfig       `yaml:"agent" mapstructure:"agent"`
	JWT         JWTConfig         `yaml:"jwt" mapstructure:"jwt"`
	Alert       AlertConfig       `yaml:"alert" mapstructure:"alert"`
	Aggregation AggregationConfig `yaml:"aggregation" mapstructure:"aggregation"`
	CORS        CORSConfig        `yaml:"cors" mapstructure:"cors"`
	RateLimit   RateLimitConfig   `yaml:"rate_limit" mapstructure:"rate_limit"`

	// 兼容字段
	Auth                AuthConfig `yaml:"auth" mapstructure:"auth"`
	RegisterSecret      string     `yaml:"register_secret" mapstructure:"register_secret"`
	HeartbeatInterval   int32      `yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"`
	TokenExpirationDays int        `yaml:"token_expiration_days" mapstructure:"token_expiration_days"`
	EnableIPQuality     bool       `yaml:"enable_ip_quality" mapstructure:"enable_ip_quality"`
}

// ServerConfig gRPC/HTTP 监听配置
type ServerConfig struct {
	HTTPPort    int    `yaml:"http_port" mapstructure:"http_port"`
	GRPCPort    int    `yaml:"grpc_port" mapstructure:"grpc_port"`
	GRPCAddress string `yaml:"grpc_address" mapstructure:"grpc_address"`
	HTTPAddress string `yaml:"http_address" mapstructure:"http_address"`
	Mode        string `yaml:"mode" mapstructure:"mode"`
}

// DatabaseConfig 数据库连接配置
type DatabaseConfig struct {
	Driver       string `yaml:"driver" mapstructure:"driver"`
	DSN          string `yaml:"dsn" mapstructure:"dsn"`
	MaxOpenConns int    `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns int    `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
}

// RedisConfig Redis 连接配置
type RedisConfig struct {
	Addr     string `yaml:"addr" mapstructure:"addr"`
	Password string `yaml:"password" mapstructure:"password"`
	DB       int    `yaml:"db" mapstructure:"db"`
}

// AgentConfig Agent 注册相关配置
type AgentConfig struct {
	RegisterSecret    string `yaml:"register_secret" mapstructure:"register_secret"`
	HeartbeatInterval int32  `yaml:"heartbeat_interval" mapstructure:"heartbeat_interval"`
	TokenExpiresDays  int    `yaml:"token_expires_days" mapstructure:"token_expires_days"`
}

// JWTConfig HTTP/WebSocket JWT 配置
type JWTConfig struct {
	Secret        string `yaml:"secret" mapstructure:"secret"`
	AdminUsername string `yaml:"admin_username" mapstructure:"admin_username"`
	AdminPassword string `yaml:"admin_password" mapstructure:"admin_password"`
}

// AlertConfig 告警引擎配置
type AlertConfig struct {
	CheckInterval int32 `yaml:"check_interval" mapstructure:"check_interval"`
}

// AggregationConfig 聚合任务配置
type AggregationConfig struct {
	Enabled              bool `yaml:"enabled" mapstructure:"enabled"`
	IntervalSeconds      int  `yaml:"interval_seconds" mapstructure:"interval_seconds"`
	IncludeCurrentBucket bool `yaml:"include_current_bucket" mapstructure:"include_current_bucket"`
}

// CORSConfig 跨域配置
type CORSConfig struct {
	AllowedOrigins []string `yaml:"allowed_origins" mapstructure:"allowed_origins"`
}

// RateLimitConfig 限流配置
type RateLimitConfig struct {
	Enabled           bool  `yaml:"enabled" mapstructure:"enabled"`
	RequestsPerSecond int64 `yaml:"requests_per_second" mapstructure:"requests_per_second"`
}

// AuthConfig 兼容旧的认证配置
type AuthConfig struct {
	JWTSecret     string `yaml:"jwt_secret" mapstructure:"jwt_secret"`
	JWTExpireHour int    `yaml:"jwt_expire_hour" mapstructure:"jwt_expire_hour"`
	AdminUsername string `yaml:"admin_username" mapstructure:"admin_username"`
	AdminPassword string `yaml:"admin_password" mapstructure:"admin_password"`
}

// DefaultConfig 返回 Server 默认配置
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPPort:    8080,
			GRPCPort:    9090,
			GRPCAddress: ":50051",
			HTTPAddress: ":8080",
			Mode:        "release",
		},
		Database: DatabaseConfig{
			Driver:       "mysql",
			MaxOpenConns: 100,
			MaxIdleConns: 10,
		},
		Redis: RedisConfig{
			Addr: "localhost:6379",
			DB:   0,
		},
		Agent: AgentConfig{
			HeartbeatInterval: DefaultHeartbeatInterval,
			TokenExpiresDays:  DefaultTokenExpirationDays,
		},
		Alert: AlertConfig{
			CheckInterval: DefaultAlertCheckInterval,
		},
		Aggregation: AggregationConfig{
			Enabled:         true,
			IntervalSeconds: 300,
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: DefaultRateLimitPerSecond,
		},
		HeartbeatInterval:   DefaultHeartbeatInterval,
		TokenExpirationDays: DefaultTokenExpirationDays,
	}
}

// Load 使用 Viper 加载配置文件。
func Load(configPath string) (*Config, error) {
	if strings.TrimSpace(configPath) == "" {
		return nil, errors.New("config path is required")
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// 默认值
	v.SetDefault("server.http_port", 8080)
	v.SetDefault("server.grpc_port", 9090)
	v.SetDefault("server.mode", "release")
	v.SetDefault("database.driver", "mysql")
	v.SetDefault("database.dsn", "")
	v.SetDefault("redis.addr", "localhost:6379")
	v.SetDefault("redis.password", "")
	v.SetDefault("agent.register_secret", "")
	v.SetDefault("agent.heartbeat_interval", DefaultHeartbeatInterval)
	v.SetDefault("agent.token_expires_days", DefaultTokenExpirationDays)
	v.SetDefault("jwt.secret", "")
	v.SetDefault("jwt.admin_username", "")
	v.SetDefault("jwt.admin_password", "")
	v.SetDefault("alert.check_interval", DefaultAlertCheckInterval)
	v.SetDefault("aggregation.enabled", true)
	v.SetDefault("aggregation.interval_seconds", 300)
	v.SetDefault("aggregation.include_current_bucket", false)
	v.SetDefault("rate_limit.enabled", true)
	v.SetDefault("rate_limit.requests_per_second", DefaultRateLimitPerSecond)
	v.SetDefault("enable_ip_quality", false)

	// 环境变量覆盖：SERVERPROBE_SERVER_HTTP_PORT 等
	v.SetEnvPrefix("SERVERPROBE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 从零值结构体反序列化，避免默认值掩盖嵌套配置
	config := &Config{}
	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	config.applyDefaults()
	config.applyCompatibility()
	return config, nil
}

// applyDefaults 填充缺失的默认值
func (c *Config) applyDefaults() {
	if c == nil {
		return
	}
	if c.Server.HTTPPort <= 0 {
		c.Server.HTTPPort = 8080
	}
	if c.Server.GRPCPort <= 0 {
		c.Server.GRPCPort = 9090
	}
	if c.Server.Mode == "" {
		c.Server.Mode = "release"
	}
	if c.Database.Driver == "" {
		c.Database.Driver = "mysql"
	}
	if c.Database.MaxOpenConns <= 0 {
		c.Database.MaxOpenConns = 100
	}
	if c.Database.MaxIdleConns <= 0 {
		c.Database.MaxIdleConns = 10
	}
	if c.Redis.Addr == "" {
		c.Redis.Addr = "localhost:6379"
	}
	if c.Alert.CheckInterval <= 0 {
		c.Alert.CheckInterval = DefaultAlertCheckInterval
	}
	if c.Aggregation.IntervalSeconds <= 0 {
		c.Aggregation.IntervalSeconds = 300
	}
	if c.RateLimit.RequestsPerSecond <= 0 {
		c.RateLimit.RequestsPerSecond = DefaultRateLimitPerSecond
	}
}

// applyCompatibility 将嵌套配置同步到兼容字段
func (c *Config) applyCompatibility() {
	if c == nil {
		return
	}

	if c.RegisterSecret == "" {
		c.RegisterSecret = c.Agent.RegisterSecret
	}
	if c.Agent.RegisterSecret == "" {
		c.Agent.RegisterSecret = c.RegisterSecret
	}

	if c.HeartbeatInterval <= 0 {
		c.HeartbeatInterval = c.Agent.HeartbeatInterval
	}
	if c.Agent.HeartbeatInterval <= 0 {
		c.Agent.HeartbeatInterval = c.HeartbeatInterval
	}

	if c.TokenExpirationDays <= 0 {
		c.TokenExpirationDays = c.Agent.TokenExpiresDays
	}
	if c.Agent.TokenExpiresDays <= 0 {
		c.Agent.TokenExpiresDays = c.TokenExpirationDays
	}

	if c.JWT.Secret == "" {
		c.JWT.Secret = c.Auth.JWTSecret
	}
	if c.Auth.JWTSecret == "" {
		c.Auth.JWTSecret = c.JWT.Secret
	}
	if c.JWT.AdminUsername == "" {
		c.JWT.AdminUsername = c.Auth.AdminUsername
	}
	if c.Auth.AdminUsername == "" {
		c.Auth.AdminUsername = c.JWT.AdminUsername
	}
	if c.JWT.AdminPassword == "" {
		c.JWT.AdminPassword = c.Auth.AdminPassword
	}
	if c.Auth.AdminPassword == "" {
		c.Auth.AdminPassword = c.JWT.AdminPassword
	}

	if c.Server.HTTPPort <= 0 {
		c.Server.HTTPPort = 8080
	}
	if c.Server.GRPCPort <= 0 {
		c.Server.GRPCPort = 9090
	}
}

// GetHeartbeatInterval 返回有效的心跳间隔
func (c *Config) GetHeartbeatInterval() int32 {
	if c == nil {
		return DefaultHeartbeatInterval
	}
	if c.HeartbeatInterval > 0 {
		return c.HeartbeatInterval
	}
	if c.Agent.HeartbeatInterval > 0 {
		return c.Agent.HeartbeatInterval
	}
	return DefaultHeartbeatInterval
}

// GetTokenExpirationDays 返回有效的 Token 有效期天数
func (c *Config) GetTokenExpirationDays() int {
	if c == nil {
		return DefaultTokenExpirationDays
	}
	if c.TokenExpirationDays > 0 {
		return c.TokenExpirationDays
	}
	if c.Agent.TokenExpiresDays > 0 {
		return c.Agent.TokenExpiresDays
	}
	return DefaultTokenExpirationDays
}

// GetRegisterSecret 返回注册密钥
func (c *Config) GetRegisterSecret() string {
	if c == nil {
		return ""
	}
	if c.RegisterSecret != "" {
		return c.RegisterSecret
	}
	return c.Agent.RegisterSecret
}

// GetJWTSecret 返回 JWT 密钥
func (c *Config) GetJWTSecret() string {
	if c == nil {
		return ""
	}
	if c.JWT.Secret != "" {
		return c.JWT.Secret
	}
	return c.Auth.JWTSecret
}

// GetAdminUsername 返回管理员用户名
func (c *Config) GetAdminUsername() string {
	if c == nil {
		return ""
	}
	if c.JWT.AdminUsername != "" {
		return c.JWT.AdminUsername
	}
	return c.Auth.AdminUsername
}

// GetAdminPassword 返回管理员密码
func (c *Config) GetAdminPassword() string {
	if c == nil {
		return ""
	}
	if c.JWT.AdminPassword != "" {
		return c.JWT.AdminPassword
	}
	return c.Auth.AdminPassword
}

// GetAlertCheckInterval 返回告警检查间隔
func (c *Config) GetAlertCheckInterval() int32 {
	if c == nil || c.Alert.CheckInterval <= 0 {
		return DefaultAlertCheckInterval
	}
	return c.Alert.CheckInterval
}

// GetAggregationInterval 返回聚合任务运行间隔
func (c *Config) GetAggregationInterval() time.Duration {
	if c == nil || c.Aggregation.IntervalSeconds <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(c.Aggregation.IntervalSeconds) * time.Second
}

// IncludeCurrentAggregationBucket 是否聚合当前未结束的时间桶（测试或低延迟场景）
func (c *Config) IncludeCurrentAggregationBucket() bool {
	return c != nil && c.Aggregation.IncludeCurrentBucket
}

// GetHTTPAddress 返回 HTTP 监听地址
func (c *Config) GetHTTPAddress() string {
	if c == nil {
		return ":8080"
	}
	if c.Server.HTTPAddress != "" {
		return c.Server.HTTPAddress
	}
	if c.Server.HTTPPort > 0 {
		return fmt.Sprintf(":%d", c.Server.HTTPPort)
	}
	return ":8080"
}

// GetGRPCAddress 返回 gRPC 监听地址
func (c *Config) GetGRPCAddress() string {
	if c == nil {
		return ":50051"
	}
	if c.Server.GRPCAddress != "" {
		return c.Server.GRPCAddress
	}
	if c.Server.GRPCPort > 0 {
		return fmt.Sprintf(":%d", c.Server.GRPCPort)
	}
	return ":50051"
}
