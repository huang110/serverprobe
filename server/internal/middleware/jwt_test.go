package middleware

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndValidateJWT(t *testing.T) {
	token, err := GenerateJWT("admin", "admin", "test-secret")
	if err != nil {
		t.Fatalf("GenerateJWT() failed: %v", err)
	}

	claims, err := ValidateJWT(token, "test-secret")
	if err != nil {
		t.Fatalf("ValidateJWT() failed: %v", err)
	}
	if claims.Username != "admin" || claims.Role != "admin" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestValidateJWTInvalidSignature(t *testing.T) {
	token, err := GenerateJWT("admin", "admin", "secret-a")
	if err != nil {
		t.Fatalf("GenerateJWT() failed: %v", err)
	}

	if _, err := ValidateJWT(token, "secret-b"); err == nil {
		t.Error("token signed with a different secret should be rejected")
	}
}

func TestValidateJWTExpired(t *testing.T) {
	token, err := GenerateJWTWithExpiry("admin", "admin", "test-secret", time.Millisecond)
	if err != nil {
		t.Fatalf("GenerateJWTWithExpiry() failed: %v", err)
	}
	time.Sleep(5 * time.Millisecond)

	if _, err := ValidateJWT(token, "test-secret"); err != ErrExpiredToken {
		t.Errorf("error = %v, want ErrExpiredToken", err)
	}
}

func TestValidateJWTWrongAlgorithm(t *testing.T) {
	claims := Claims{
		Username: "admin",
		Role:     "admin",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	signed, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("SignedString() failed: %v", err)
	}

	if _, err := ValidateJWT(signed, "test-secret"); err != ErrInvalidToken {
		t.Errorf("error = %v, want ErrInvalidToken", err)
	}
}

func TestValidateJWTValidation(t *testing.T) {
	if _, err := ValidateJWT("", "secret"); err != ErrInvalidToken {
		t.Errorf("empty token error = %v, want ErrInvalidToken", err)
	}
	if _, err := ValidateJWT("token", ""); err != ErrInvalidToken {
		t.Errorf("empty secret error = %v, want ErrInvalidToken", err)
	}
	if _, err := GenerateJWT("admin", "admin", ""); err != ErrInvalidToken {
		t.Errorf("empty secret error = %v, want ErrInvalidToken", err)
	}
}
