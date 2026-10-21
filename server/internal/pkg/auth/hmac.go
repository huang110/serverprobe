package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// SignatureWindowSeconds 允许的时间戳误差（秒）
	SignatureWindowSeconds int64 = 300

	// signatureSeparator 签名原文分隔符，格式为 agentID:timestamp
	signatureSeparator = ":"
)

// GenerateSignature 生成 HMAC-SHA256 签名。
func GenerateSignature(agentID string, timestamp int64, token string) string {
	message := fmt.Sprintf("%s%s%d", agentID, signatureSeparator, timestamp)
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature 使用常量时间比较验证签名。
func VerifySignature(agentID string, timestamp int64, token, signature string) bool {
	expected, err := hex.DecodeString(GenerateSignature(agentID, timestamp, token))
	if err != nil {
		return false
	}
	actual, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	return hmac.Equal(expected, actual)
}

// ValidateTimestamp 验证时间戳是否在 ±5 分钟窗口内。
// 使用区间判断避免对极端值求绝对值时溢出。
func ValidateTimestamp(timestamp int64) bool {
	now := time.Now().Unix()
	return timestamp >= now-SignatureWindowSeconds && timestamp <= now+SignatureWindowSeconds
}
