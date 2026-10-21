package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/huang110/serverprobe/server/internal/config"
	"github.com/huang110/serverprobe/server/internal/middleware"
)

func newAuthTestRouter(cfg *config.Config) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/api/auth/login", NewAuthHandler(cfg).Login)
	return router
}

func TestLoginSuccess(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.JWT.Secret = "jwt-secret"
	cfg.JWT.AdminUsername = "admin"
	cfg.JWT.AdminPassword = "admin123"

	router := newAuthTestRouter(cfg)
	recorder := performRequest(t, router, http.MethodPost, "/api/auth/login",
		[]byte(`{"username":"admin","password":"admin123"}`))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	response := decodeResponse(t, recorder)
	data := response["data"].(map[string]interface{})
	token := data["token"].(string)
	if _, err := middleware.ValidateJWT(token, "jwt-secret"); err != nil {
		t.Errorf("generated token is invalid: %v", err)
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.JWT.Secret = "jwt-secret"
	cfg.JWT.AdminUsername = "admin"
	cfg.JWT.AdminPassword = "admin123"

	router := newAuthTestRouter(cfg)
	recorder := performRequest(t, router, http.MethodPost, "/api/auth/login",
		[]byte(`{"username":"admin","password":"wrong"}`))
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", recorder.Code)
	}
}

func TestLoginInvalidBody(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.JWT.Secret = "jwt-secret"
	cfg.JWT.AdminUsername = "admin"
	cfg.JWT.AdminPassword = "admin123"

	router := newAuthTestRouter(cfg)
	recorder := performRequest(t, router, http.MethodPost, "/api/auth/login", []byte(`not-json`))
	if recorder.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", recorder.Code)
	}
}

func TestLoginNotConfigured(t *testing.T) {
	router := newAuthTestRouter(config.DefaultConfig())
	recorder := performRequest(t, router, http.MethodPost, "/api/auth/login",
		[]byte(`{"username":"admin","password":"admin"}`))
	if recorder.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", recorder.Code)
	}
}
