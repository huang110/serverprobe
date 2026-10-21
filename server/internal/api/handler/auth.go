package handler

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/middleware"
)

// AuthHandler 认证接口
type AuthHandler struct {
	config *config.Config
}

// NewAuthHandler 创建认证接口
func NewAuthHandler(cfg *config.Config) *AuthHandler {
	return &AuthHandler{config: cfg}
}

// Login POST /api/auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		respondError(c, http.StatusBadRequest, "invalid request")
		return
	}

	if h.config == nil {
		respondError(c, http.StatusInternalServerError, "auth is not configured")
		return
	}

	expectedUsername := h.config.GetAdminUsername()
	expectedPassword := h.config.GetAdminPassword()
	if expectedUsername == "" || expectedPassword == "" {
		respondError(c, http.StatusInternalServerError, "auth is not configured")
		return
	}

	if !secureCompare(request.Username, expectedUsername) || !secureCompare(request.Password, expectedPassword) {
		respondError(c, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := middleware.GenerateJWT(request.Username, "admin", h.config.GetJWTSecret())
	if err != nil {
		respondError(c, http.StatusInternalServerError, "failed to generate token")
		return
	}

	respondOK(c, gin.H{"token": token})
}

func secureCompare(actual, expected string) bool {
	if actual == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}
