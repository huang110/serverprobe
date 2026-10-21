package client

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	// SignatureWindowSeconds 允许的时间戳误差（秒）。
	// 服务端和客户端时钟允许相差 ±5 分钟，超出范围视为重放请求。
	SignatureWindowSeconds int64 = 300

	// signatureSeparator 签名原文的分隔符，格式为 agentID:timestamp
	signatureSeparator = ":"
)

// GenerateSignature 生成 HMAC-SHA256 签名。
// 签名原文固定为 "agentID:timestamp"，返回小写十六进制字符串。
// 该格式与 proto 中的 AgentId、Timestamp、Signature 字段一一对应。
func GenerateSignature(agentID string, timestamp int64, token string) string {
	message := buildSignatureMessage(agentID, timestamp)
	mac := hmac.New(sha256.New, []byte(token))
	_, _ = mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifySignature 校验签名和请求时间戳。
// 签名比较使用 hmac.Equal，避免时序攻击。
func VerifySignature(agentID string, timestamp int64, token, signature string) bool {
	if !ValidateTimestamp(timestamp) {
		return false
	}

	expected := GenerateSignature(agentID, timestamp, token)

	// 将十六进制字符串还原为字节后比较，兼容大小写形式
	expectedBytes, err := hex.DecodeString(expected)
	if err != nil {
		return false
	}
	actualBytes, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}

	return hmac.Equal(expectedBytes, actualBytes)
}

// ValidateTimestamp 验证时间戳是否在允许的时间窗口内。
// 使用 now±window 的区间判断，避免对 math.MinInt64 等极端值求绝对值时溢出。
func ValidateTimestamp(timestamp int64) bool {
	now := time.Now().Unix()
	return timestamp >= now-SignatureWindowSeconds && timestamp <= now+SignatureWindowSeconds
}

// buildSignatureMessage 构造签名原文：agentID:timestamp
func buildSignatureMessage(agentID string, timestamp int64) string {
	return fmt.Sprintf("%s%s%d", agentID, signatureSeparator, timestamp)
}
