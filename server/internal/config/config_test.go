package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}
	return path
}

func TestLoadConfig(t *testing.T) {
	path := writeConfigFile(t, `
server:
  http_port: 9091
  grpc_port: 9092
database:
  driver: sqlite
  dsn: "test.db"
redis:
  addr: "localhost:6380"
  db: 1
agent:
  register_secret: "secret"
  heartbeat_interval: 45
  token_expires_days: 10
jwt:
  secret: "jwt-secret"
  admin_username: "admin"
  admin_password: "admin123"
alert:
  check_interval: 15
cors:
  allowed_origins:
    - "http://localhost:3000"
rate_limit:
  enabled: true
  requests_per_second: 50
enable_ip_quality: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.HTTPPort != 9091 || cfg.Server.GRPCPort != 9092 {
		t.Errorf("unexpected server config: %+v", cfg.Server)
	}
	if cfg.Database.Driver != "sqlite" || cfg.Database.DSN != "test.db" {
		t.Errorf("unexpected database config: %+v", cfg.Database)
	}
	if cfg.Redis.Addr != "localhost:6380" || cfg.Redis.DB != 1 {
		t.Errorf("unexpected redis config: %+v", cfg.Redis)
	}
	if cfg.GetRegisterSecret() != "secret" || cfg.GetHeartbeatInterval() != 45 {
		t.Errorf("unexpected agent config: %+v", cfg.Agent)
	}
	if cfg.GetTokenExpirationDays() != 10 {
		t.Errorf("token expiration days = %d, want 10", cfg.GetTokenExpirationDays())
	}
	if cfg.GetJWTSecret() != "jwt-secret" || cfg.GetAdminUsername() != "admin" || cfg.GetAdminPassword() != "admin123" {
		t.Errorf("unexpected jwt config: %+v", cfg.JWT)
	}
	if cfg.GetAlertCheckInterval() != 15 {
		t.Errorf("alert check interval = %d, want 15", cfg.GetAlertCheckInterval())
	}
	if len(cfg.CORS.AllowedOrigins) != 1 || cfg.CORS.AllowedOrigins[0] != "http://localhost:3000" {
		t.Errorf("unexpected cors config: %+v", cfg.CORS)
	}
	if !cfg.RateLimit.Enabled || cfg.RateLimit.RequestsPerSecond != 50 {
		t.Errorf("unexpected rate limit config: %+v", cfg.RateLimit)
	}
	if !cfg.EnableIPQuality {
		t.Error("EnableIPQuality should be true")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	path := writeConfigFile(t, "server: {}\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Server.HTTPPort != 8080 || cfg.Server.GRPCPort != 9090 {
		t.Errorf("unexpected default ports: %+v", cfg.Server)
	}
	if cfg.GetHeartbeatInterval() != DefaultHeartbeatInterval {
		t.Errorf("heartbeat interval = %d, want %d", cfg.GetHeartbeatInterval(), DefaultHeartbeatInterval)
	}
	if cfg.GetTokenExpirationDays() != DefaultTokenExpirationDays {
		t.Errorf("token expiration days = %d, want %d", cfg.GetTokenExpirationDays(), DefaultTokenExpirationDays)
	}
	if cfg.GetAlertCheckInterval() != DefaultAlertCheckInterval {
		t.Errorf("alert check interval = %d, want %d", cfg.GetAlertCheckInterval(), DefaultAlertCheckInterval)
	}
	if cfg.RateLimit.RequestsPerSecond != DefaultRateLimitPerSecond {
		t.Errorf("rate limit = %d, want %d", cfg.RateLimit.RequestsPerSecond, DefaultRateLimitPerSecond)
	}
}

func TestLoadConfigEnvOverride(t *testing.T) {
	path := writeConfigFile(t, "server:\n  http_port: 8080\n")
	t.Setenv("SERVERPROBE_SERVER_HTTP_PORT", "7777")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Server.HTTPPort != 7777 {
		t.Errorf("HTTP port = %d, want 7777 from env", cfg.Server.HTTPPort)
	}
}

func TestLoadConfigMissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "missing.yaml")); err == nil {
		t.Error("missing config file should return an error")
	}
	if _, err := Load(""); err == nil {
		t.Error("empty config path should return an error")
	}
}
