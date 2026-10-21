package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestJWTAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)

	token, err := GenerateJWT("admin", "admin", "test-secret")
	if err != nil {
		t.Fatalf("GenerateJWT() failed: %v", err)
	}

	router := gin.New()
	router.GET("/protected", JWTAuth("test-secret"), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"valid", "Bearer " + token, http.StatusOK},
		{"missing", "", http.StatusUnauthorized},
		{"invalid", "Bearer invalid", http.StatusUnauthorized},
		{"wrong secret", "Bearer " + token, http.StatusUnauthorized},
	}

	for _, test := range tests {
		if test.name == "wrong secret" {
			otherRouter := gin.New()
			otherRouter.GET("/protected", JWTAuth("other-secret"), func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"ok": true})
			})
			request := httptest.NewRequest(http.MethodGet, "/protected", nil)
			request.Header.Set("Authorization", test.header)
			recorder := httptest.NewRecorder()
			otherRouter.ServeHTTP(recorder, request)
			if recorder.Code != test.want {
				t.Errorf("%s: status = %d, want %d", test.name, recorder.Code, test.want)
			}
			continue
		}

		request := httptest.NewRequest(http.MethodGet, "/protected", nil)
		if test.header != "" {
			request.Header.Set("Authorization", test.header)
		}
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != test.want {
			t.Errorf("%s: status = %d, want %d", test.name, recorder.Code, test.want)
		}
	}
}
