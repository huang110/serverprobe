package websocket

import (
	"log"
	"sync"
)

// broadcastBuffer 广播消息缓冲大小
const broadcastBuffer = 256

// Hub WebSocket 连接池
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}

	register   chan *Client
	unregister chan *Client
	broadcast  chan []byte

	stopCh   chan struct{}
	doneCh   chan struct{}
	stopOnce sync.Once
}

// NewHub 创建连接池
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan []byte, broadcastBuffer),
		stopCh:     make(chan struct{}),
		doneCh:     make(chan struct{}),
	}
}

// Run 启动连接池事件循环，阻塞运行直到 Stop
func (h *Hub) Run() {
	defer close(h.doneCh)

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.removeClient(client)
		case message := <-h.broadcast:
			h.dispatch(message)
		case <-h.stopCh:
			h.shutdown()
			return
		}
	}
}

// Register 注册客户端
func (h *Hub) Register(client *Client) {
	select {
	case h.register <- client:
	case <-h.stopCh:
	}
}

// Unregister 注销客户端
func (h *Hub) Unregister(client *Client) {
	select {
	case h.unregister <- client:
	case <-h.stopCh:
	}
}

// Broadcast 广播消息给所有客户端
func (h *Hub) Broadcast(message []byte) {
	select {
	case h.broadcast <- message:
	case <-h.stopCh:
	}
}

// ClientCount 返回当前连接数
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Stop 停止连接池并关闭所有客户端
func (h *Hub) Stop() {
	h.stopOnce.Do(func() {
		close(h.stopCh)
	})
	<-h.doneCh
}

// removeClient 从连接池移除客户端并关闭其发送通道
func (h *Hub) removeClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.clients[client]; !ok {
		return
	}
	delete(h.clients, client)
	client.closeSend()
}

// dispatch 向所有客户端发送消息，慢客户端会被移除
func (h *Hub) dispatch(message []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		select {
		case client.send <- message:
		default:
			delete(h.clients, client)
			client.closeSend()
			log.Printf("warning: removed slow websocket client")
		}
	}
}

// shutdown 关闭所有客户端
func (h *Hub) shutdown() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		delete(h.clients, client)
		client.closeSend()
	}
}
