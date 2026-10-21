package client

import (
	"math"
	"strings"
	"testing"
	"time"
)

// TestGenerateSignature 使用独立工具生成的 HMAC 结果验证签名格式和算法
func TestGenerateSignature(t *testing.T) {
	const (
		agentID   = "agent-123"
		timestamp = int64(1700000000)
		token     = "secret-token"
		expected  = "e30b335d2aa4330ec066429d103dca708858d5513a5b86b04ecd04eea899ad9d"
	)

	if got := GenerateSignature(agentID, timestamp, token); got != expected {
		t.Errorf("GenerateSignature() = %q, want %q", got, expected)
	}
}

// TestBuildSignatureMessage 验证签名原文格式为 agentID:timestamp
func TestBuildSignatureMessage(t *testing.T) {
	if got := buildSignatureMessage("agent-1", 42); got != "agent-1:42" {
		t.Errorf("buildSignatureMessage() = %q, want %q", got, "agent-1:42")
	}
}

// TestVerifySignature 验证正确签名、错误签名和过期时间戳
func TestVerifySignature(t *testing.T) {
	now := time.Now().Unix()
	signature := GenerateSignature("agent-1", now, "token")

	if !VerifySignature("agent-1", now, "token", signature) {
		t.Error("valid signature should be accepted")
	}

	// hex.DecodeString 支持大写，因此大写签名也应通过
	if !VerifySignature("agent-1", now, "token", strings.ToUpper(signature)) {
		t.Error("uppercase hex signature should be accepted")
	}

	if VerifySignature("agent-1", now, "wrong-token", signature) {
		t.Error("signature generated with a different token should be rejected")
	}

	if VerifySignature("agent-2", now, "token", signature) {
		t.Error("signature generated for a different agent should be rejected")
	}

	if VerifySignature("agent-1", now, "token", "not-hex") {
		t.Error("invalid hex signature should be rejected")
	}

	expired := now - 2*SignatureWindowSeconds
	if VerifySignature("agent-1", expired, "token", GenerateSignature("agent-1", expired, "token")) {
		t.Error("expired timestamp should be rejected")
	}
}

// TestValidateTimestamp 验证时间戳窗口和极端值处理
func TestValidateTimestamp(t *testing.T) {
	now := time.Now().Unix()

	valid := []int64{
		now,
		now - SignatureWindowSeconds + 1,
		now + SignatureWindowSeconds - 1,
	}
	for _, timestamp := range valid {
		if !ValidateTimestamp(timestamp) {
			t.Errorf("timestamp %d should be valid", timestamp)
		}
	}

	invalid := []int64{
		now - SignatureWindowSeconds - 1,
		now + SignatureWindowSeconds + 1,
		now - 2*SignatureWindowSeconds,
		0,
		-1,
		math.MinInt64,
		math.MaxInt64,
	}
	for _, timestamp := range invalid {
		if ValidateTimestamp(timestamp) {
			t.Errorf("timestamp %d should be invalid", timestamp)
		}
	}
}
