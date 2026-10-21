package model

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestAgentTokenNotSerialized 验证 Token 不会出现在 JSON 输出中
func TestAgentTokenNotSerialized(t *testing.T) {
	agent := Agent{
		ID:              "agent-1",
		Hostname:        "node-1",
		Token:           "super-secret-token",
		TokenExpiresAt:  1700000000,
		LastHeartbeatAt: 1700000001,
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("json.Marshal() failed: %v", err)
	}

	output := string(data)
	if strings.Contains(output, "super-secret-token") {
		t.Error("token value should not be serialized")
	}
	if strings.Contains(output, `"token":`) {
		t.Error("token field should not be serialized")
	}
	if !strings.Contains(output, `"token_expires_at":1700000000`) {
		t.Errorf("token_expires_at should be serialized: %s", output)
	}
}
