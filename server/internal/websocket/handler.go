package websocket

import (
	"net/http"

	"github.com/gin-gonic/gin"
	gwebsocket "github.com/gorilla/websocket"

	"github.com/huang110/serverprobe/server/internal/middleware"
)

var upgrader = gwebsocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

// HandleWebSocket GET /ws?token=xxx
func HandleWebSocket(hub *Hub, jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := c.Query("token")
		claims, err := middleware.ValidateJWT(token, jwtSecret)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		var expiresAt int64
		if claims.ExpiresAt != nil {
			expiresAt = claims.ExpiresAt.Unix()
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}

		client := NewClient(hub, conn, expiresAt)
		hub.Register(client)

		go client.writePump()
		go client.readPump()
	}
}
