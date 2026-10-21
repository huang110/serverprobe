package handler

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHealthEndpoints(t *testing.T) {
	api := setupTestAPI(t)
	handler := NewHealthHandler(api.db, api.redis)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/health", handler.Health)
	router.GET("/api/health/ready", handler.Ready)
	router.GET("/api/health/metrics", handler.Metrics)

	recorder := performRequest(t, router, http.MethodGet, "/api/health", nil)
	if recorder.Code != http.StatusOK {
		t.Errorf("health status = %d, want 200", recorder.Code)
	}

	recorder = performRequest(t, router, http.MethodGet, "/api/health/ready", nil)
	if recorder.Code != http.StatusOK {
		t.Errorf("ready status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}

	recorder = performRequest(t, router, http.MethodGet, "/api/health/metrics", nil)
	if recorder.Code != http.StatusOK {
		t.Errorf("metrics status = %d, want 200", recorder.Code)
	}
	if recorder.Header().Get("Content-Type") != "text/plain; version=0.0.4" {
		t.Errorf("unexpected content type: %s", recorder.Header().Get("Content-Type"))
	}
}

func TestHealthNotReady(t *testing.T) {
	api := setupTestAPI(t)
	handler := NewHealthHandler(api.db, nil)

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/health/ready", handler.Ready)

	recorder := performRequest(t, router, http.MethodGet, "/api/health/ready", nil)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Errorf("ready status = %d, want 503", recorder.Code)
	}
}
