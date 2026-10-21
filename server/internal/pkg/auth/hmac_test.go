package auth

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestGenerateSignature(t *testing.T) {
	const expected = "e30b335d2aa4330ec066429d103dca708858d5513a5b86b04ecd04eea899ad9d"
	if got := GenerateSignature("agent-123", 1700000000, "secret-token"); got != expected {
		t.Errorf("GenerateSignature() = %q, want %q", got, expected)
	}
}

func TestVerifySignature(t *testing.T) {
	now := time.Now().Unix()
	signature := GenerateSignature("agent-1", now, "token")

	if !VerifySignature("agent-1", now, "token", signature) {
		t.Error("valid signature should be accepted")
	}
	if !VerifySignature("agent-1", now, "token", strings.ToUpper(signature)) {
		t.Error("uppercase hex signature should be accepted")
	}
	if VerifySignature("agent-1", now, "wrong-token", signature) {
		t.Error("wrong token should be rejected")
	}
	if VerifySignature("agent-2", now, "token", signature) {
		t.Error("wrong agent id should be rejected")
	}
	if VerifySignature("agent-1", now, "token", "not-hex") {
		t.Error("invalid hex should be rejected")
	}
}

func TestValidateTimestamp(t *testing.T) {
	now := time.Now().Unix()

	valid := []int64{now, now - SignatureWindowSeconds + 1, now + SignatureWindowSeconds - 1}
	for _, timestamp := range valid {
		if !ValidateTimestamp(timestamp) {
			t.Errorf("timestamp %d should be valid", timestamp)
		}
	}

	invalid := []int64{
		now - SignatureWindowSeconds - 1,
		now + SignatureWindowSeconds + 1,
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
