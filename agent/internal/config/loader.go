package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

const (
	DefaultConfigName = "agent.yaml"
	DefaultConfigPath = "/etc/serverprobe"
	HomeConfigPath    = ".serverprobe"
)

// Loader 配置加载器
type Loader struct {
	configFile string
	config     *Config
}

// NewLoader 创建配置加载器
func NewLoader(configFile string) *Loader {
	return &Loader{
		configFile: configFile,
		config:     DefaultConfig(),
	}
}

// Load 加载配置文件
func (l *Loader) Load() (*Config, error) {
	// 如果指定了配置文件，直接加载
	if l.configFile != "" {
		return l.loadFromFile(l.configFile)
	}

	// 按优先级查找配置文件
	configPaths := []string{
		// 1. 当前目录
		filepath.Join(".", DefaultConfigName),
		// 2. 用户主目录
		filepath.Join(os.Getenv("HOME"), HomeConfigPath, DefaultConfigName),
		// 3. 系统配置目录
		filepath.Join(DefaultConfigPath, DefaultConfigName),
	}

	for _, path := range configPaths {
		if _, err := os.Stat(path); err == nil {
			return l.loadFromFile(path)
		}
	}

	// 如果没有找到配置文件，返回默认配置
	return l.config, nil
}

// loadFromFile 从文件加载配置
func (l *Loader) loadFromFile(file string) (*Config, error) {
	viper.SetConfigFile(file)
	viper.SetConfigType("yaml")

	// 支持环境变量覆盖
	viper.AutomaticEnv()
	viper.SetEnvPrefix("SERVERPROBE")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := DefaultConfig()
	// viper 默认使用 mapstructure 标签，而配置结构体使用 yaml 标签，
	// 这里显式指定 TagName 以正确映射 snake_case 字段（如 register_secret）。
	if err := viper.Unmarshal(config, func(decoderConfig *mapstructure.DecoderConfig) {
		decoderConfig.TagName = "yaml"
	}); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// 处理环境变量中的 register_secret
	if config.Server.RegisterSecret == "" {
		config.Server.RegisterSecret = os.Getenv("SERVERPROBE_REGISTER_SECRET")
	}

	l.config = config
	return config, nil
}

// Save 保存配置到文件
func (l *Loader) Save(config *Config) error {
	configFile := l.configFile
	if configFile == "" {
		// 默认保存到用户主目录
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "."
		}
		configDir := filepath.Join(homeDir, HomeConfigPath)
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		configFile = filepath.Join(configDir, DefaultConfigName)
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// UpdateAgent 更新 Agent 配置并保存
func (l *Loader) UpdateAgent(agentID, token string, expiresAt int64) error {
	l.config.Agent.ID = agentID
	l.config.Agent.Token = token
	l.config.Agent.TokenExpiresAt = expiresAt
	return l.Save(l.config)
}

// UpdateHeartbeatInterval 更新心跳间隔
func (l *Loader) UpdateHeartbeatInterval(interval int32) error {
	l.config.Monitor.HeartbeatInterval = int(interval)
	return l.Save(l.config)
}

// GetConfig 获取配置
func (l *Loader) GetConfig() *Config {
	return l.config
}
