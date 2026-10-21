package websocket

import (
	"encoding/json"
	"sync"
	"time"

	gwebsocket "github.com/gorilla/websocket"
)

const (
	// writeWait 单次写超时
	writeWait = 10 * time.Second

	// pongWait 客户端最长无消息时间
	pongWait = 60 * time.Second

	// pingPeriod 服务端保活间隔
	pingPeriod = 30 * time.Second

	// maxMessageSize 客户端消息大小上限
	maxMessageSize = 1024

	// sendBuffer 客户端发送缓冲
	sendBuffer = 256
)

// inboundMessage 客户端 → 服务端消息
type inboundMessage struct {
	Type string `json:"type"`
}

// outboundMessage 服务端 → 客户端消息
type outboundMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data,omitempty"`
}

// Client 单个 WebSocket 客户端
type Client struct {
	hub       *Hub
	conn      *gwebsocket.Conn
	send      chan []byte
	expiresAt int64

	closeOnce sync.Once
}

// NewClient 创建客户端，expiresAt 为 JWT 过期时间（Unix 秒，0 表示不过期）
func NewClient(hub *Hub, conn *gwebsocket.Conn, expiresAt int64) *Client {
	return &Client{
		hub:       hub,
		conn:      conn,
		send:      make(chan []byte, sendBuffer),
		expiresAt: expiresAt,
	}
}

// closeSend 关闭发送通道，可重复调用
func (c *Client) closeSend() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}

// tokenExpired 判断 JWT 是否已过期
func (c *Client) tokenExpired() bool {
	return c.expiresAt > 0 && time.Now().Unix() >= c.expiresAt
}

// enqueue 非阻塞入队，避免慢客户端阻塞事件循环
func (c *Client) enqueue(message []byte) {
	select {
	case c.send <- message:
	default:
	}
}

// readPump 读取客户端消息，超过 pongWait 未收到消息则断开
func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)

	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		_, payload, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var message inboundMessage
		if err := json.Unmarshal(payload, &message); err != nil {
			continue
		}
		if message.Type == "ping" {
			c.enqueue(pongMessage())
		}
	}
}

// writePump 发送队列消息，并每 30 秒发送一次 pong 保活
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		if c.tokenExpired() {
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			_ = c.conn.WriteMessage(gwebsocket.CloseMessage, gwebsocket.FormatCloseMessage(gwebsocket.ClosePolicyViolation, "token expired"))
			return
		}

		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(gwebsocket.CloseMessage, gwebsocket.FormatCloseMessage(gwebsocket.CloseNormalClosure, ""))
				return
			}
			if err := c.conn.WriteMessage(gwebsocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(gwebsocket.TextMessage, pongMessage()); err != nil {
				return
			}
		}
	}
}

// pongMessage 构造 pong 消息
func pongMessage() []byte {
	payload, _ := json.Marshal(outboundMessage{
		Type: "pong",
		Data: map[string]int64{"timestamp": time.Now().Unix()},
	})
	return payload
}
