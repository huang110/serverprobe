package middleware

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	// ErrInvalidToken Token 无效
	ErrInvalidToken = errors.New("invalid token")

	// ErrExpiredToken Token 已过期
	ErrExpiredToken = errors.New("token expired")
)

// DefaultJWTExpiry 默认 JWT 有效期
const DefaultJWTExpiry = 24 * time.Hour

// Claims JWT 载荷
type Claims struct {
	Username string `json:"sub"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// GenerateJWT 生成默认 24 小时有效期的 JWT
func GenerateJWT(username, role, secret string) (string, error) {
	return GenerateJWTWithExpiry(username, role, secret, DefaultJWTExpiry)
}

// GenerateJWTWithExpiry 生成指定有效期的 JWT
func GenerateJWTWithExpiry(username, role, secret string, expiry time.Duration) (string, error) {
	if secret == "" {
		return "", ErrInvalidToken
	}
	if expiry <= 0 {
		expiry = DefaultJWTExpiry
	}

	now := time.Now()
	claims := Claims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// ValidateJWT 验证 JWT 并返回 Claims
func ValidateJWT(tokenString, secret string) (*Claims, error) {
	if tokenString == "" || secret == "" {
		return nil, ErrInvalidToken
	}

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}
	return claims, nil
}
