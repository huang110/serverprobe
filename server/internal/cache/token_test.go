package cache

import (
	"strings"
	"testing"
	"time"
)

func TestTokenBlacklist(t *testing.T) {
	client, server := setupTestRedis(t)

	if err := client.AddTokenToBlacklist("agent-1", "old-token", time.Minute); err != nil {
		t.Fatalf("AddTokenToBlacklist() failed: %v", err)
	}

	blacklisted, err := client.IsTokenBlacklisted("agent-1", "old-token")
	if err != nil {
		t.Fatalf("IsTokenBlacklisted() failed: %v", err)
	}
	if !blacklisted {
		t.Error("old token should be blacklisted")
	}

	blacklisted, err = client.IsTokenBlacklisted("agent-1", "new-token")
	if err != nil {
		t.Fatalf("IsTokenBlacklisted() failed: %v", err)
	}
	if blacklisted {
		t.Error("new token should not be blacklisted")
	}

	// Redis key 中只能出现 Token 的哈希，不能出现明文
	keys, err := client.Client().Keys(client.Context(), "token:blacklist:*").Result()
	if err != nil {
		t.Fatalf("Keys() failed: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("blacklist key count = %d, want 1", len(keys))
	}
	if strings.Contains(keys[0], "old-token") {
		t.Errorf("blacklist key should not contain plaintext token: %s", keys[0])
	}
	if !strings.Contains(keys[0], hashToken("old-token")) {
		t.Errorf("blacklist key should contain token hash: %s", keys[0])
	}

	if ttl := server.TTL(keys[0]); ttl <= 0 || ttl > time.Minute {
		t.Errorf("blacklist TTL = %s, want 0 < TTL <= 1m", ttl)
	}

	if err := client.RemoveTokenFromBlacklist("agent-1", "old-token"); err != nil {
		t.Fatalf("RemoveTokenFromBlacklist() failed: %v", err)
	}
	blacklisted, _ = client.IsTokenBlacklisted("agent-1", "old-token")
	if blacklisted {
		t.Error("token should not be blacklisted after removal")
	}
}

func TestTokenBlacklistDefaultTTL(t *testing.T) {
	client, server := setupTestRedis(t)

	if err := client.AddTokenToBlacklist("agent-1", "token", 0); err != nil {
		t.Fatalf("AddTokenToBlacklist() failed: %v", err)
	}

	key := tokenBlacklistKey("agent-1", "token")
	if ttl := server.TTL(key); ttl != tokenBlacklistTTL {
		t.Errorf("default TTL = %s, want %s", ttl, tokenBlacklistTTL)
	}
}

func TestTokenBlacklistValidation(t *testing.T) {
	client, _ := setupTestRedis(t)

	if err := client.AddTokenToBlacklist("", "token", time.Minute); err == nil {
		t.Error("empty agent id should return an error")
	}
	if err := client.AddTokenToBlacklist("agent-1", "", time.Minute); err == nil {
		t.Error("empty token should return an error")
	}
	if _, err := client.IsTokenBlacklisted("", "token"); err == nil {
		t.Error("empty agent id should return an error")
	}

	blacklisted, err := client.IsTokenBlacklisted("agent-1", "")
	if err != nil {
		t.Fatalf("empty token check should not fail: %v", err)
	}
	if blacklisted {
		t.Error("empty token should not be blacklisted")
	}
}
