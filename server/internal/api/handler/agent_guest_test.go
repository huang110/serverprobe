package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/huang110/serverprobe/server/internal/middleware"
)

func TestMaskIP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"36.232.104.106", "36.232.*.*"},
		{"10.0.0.5", "10.0.*.*"},
		{"2001:db8:abcd::1", "2001:db8:*:*:*:*:*:*"},
		{"", ""},
		{"not-an-ip", ""},
	}

	for _, test := range tests {
		if got := maskIP(test.input); got != test.want {
			t.Errorf("maskIP(%q) = %q, want %q", test.input, got, test.want)
		}
	}
}

func TestAgentGuestMasking(t *testing.T) {
	api := setupTestAPI(t)
	agent := createAgent(t, api.db, "agent-1", "web-1", "online")
	if err := api.db.Model(agent).Updates(map[string]interface{}{
		"ip":          "36.232.104.106",
		"internal_ip": "10.0.0.5",
	}).Error; err != nil {
		t.Fatalf("failed to update agent IP: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	agentHandler := NewAgentHandler(api.db, api.redis)
	router.GET("/api/agents", middleware.OptionalJWTAuth("test-secret"), agentHandler.ListAgents)
	router.GET("/api/agents/:id", middleware.OptionalJWTAuth("test-secret"), agentHandler.GetAgent)

	guestRequest := httptest.NewRequest(http.MethodGet, "/api/agents", nil)
	guestRecorder := httptest.NewRecorder()
	router.ServeHTTP(guestRecorder, guestRequest)
	if guestRecorder.Code != http.StatusOK {
		t.Fatalf("guest list status = %d", guestRecorder.Code)
	}
	guestResponse := decodeResponse(t, guestRecorder)
	guestData := guestResponse["data"].(map[string]interface{})
	guestAgent := guestData["agents"].([]interface{})[0].(map[string]interface{})
	if guestAgent["ip"] != "36.232.*.*" {
		t.Errorf("guest ip = %v, want masked", guestAgent["ip"])
	}
	if guestAgent["internal_ip"] != "" {
		t.Errorf("guest internal_ip = %v, want empty", guestAgent["internal_ip"])
	}

	token, err := middleware.GenerateJWT("admin", "admin", "test-secret")
	if err != nil {
		t.Fatalf("GenerateJWT() failed: %v", err)
	}
	authRequest := httptest.NewRequest(http.MethodGet, "/api/agents/agent-1", nil)
	authRequest.Header.Set("Authorization", "Bearer "+token)
	authRecorder := httptest.NewRecorder()
	router.ServeHTTP(authRecorder, authRequest)
	if authRecorder.Code != http.StatusOK {
		t.Fatalf("authenticated detail status = %d", authRecorder.Code)
	}
	authResponse := decodeResponse(t, authRecorder)
	authData := authResponse["data"].(map[string]interface{})
	authAgent := authData["agent"].(map[string]interface{})
	if authAgent["ip"] != "36.232.104.106" {
		t.Errorf("authenticated ip = %v, want full IP", authAgent["ip"])
	}
	if authAgent["internal_ip"] != "10.0.0.5" {
		t.Errorf("authenticated internal_ip = %v, want 10.0.0.5", authAgent["internal_ip"])
	}

	guestDetailRequest := httptest.NewRequest(http.MethodGet, "/api/agents/agent-1", nil)
	guestDetailRecorder := httptest.NewRecorder()
	router.ServeHTTP(guestDetailRecorder, guestDetailRequest)
	guestDetailResponse := decodeResponse(t, guestDetailRecorder)
	guestDetailData := guestDetailResponse["data"].(map[string]interface{})
	guestDetailAgent := guestDetailData["agent"].(map[string]interface{})
	if guestDetailAgent["ip"] != "36.232.*.*" || guestDetailAgent["internal_ip"] != "" {
		t.Errorf("guest detail should be masked: %+v", guestDetailAgent)
	}
}
