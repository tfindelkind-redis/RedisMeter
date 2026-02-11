package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketHub manages WebSocket connections and message broadcasting.
type WebSocketHub struct {
	// Registered connections by benchmark ID
	connections map[string]map[*WebSocketConn]bool
	mu          sync.RWMutex

	// Channels
	register   chan *connRegistration
	unregister chan *WebSocketConn
	broadcast  chan *broadcastMessage

	// Upgrader for WebSocket connections
	upgrader websocket.Upgrader
}

// WebSocketConn represents a WebSocket connection.
type WebSocketConn struct {
	hub      *WebSocketHub
	conn     *websocket.Conn
	send     chan []byte
	benchID  string
	clientID string
}

type connRegistration struct {
	conn    *WebSocketConn
	benchID string
}

type broadcastMessage struct {
	benchID string
	message []byte
}

// WebSocketMessage represents a message sent over WebSocket.
type WebSocketMessage struct {
	Type      string      `json:"type"`
	BenchID   string      `json:"bench_id,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// ProgressData represents benchmark progress data.
type ProgressData struct {
	Progress    float64 `json:"progress"`
	OpsPerSec   float64 `json:"ops_per_sec,omitempty"`
	AvgLatency  float64 `json:"avg_latency,omitempty"`
	P99Latency  float64 `json:"p99_latency,omitempty"`
	ElapsedTime float64 `json:"elapsed_time,omitempty"`
}

// NewWebSocketHub creates a new WebSocket hub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		connections: make(map[string]map[*WebSocketConn]bool),
		register:    make(chan *connRegistration),
		unregister:  make(chan *WebSocketConn),
		broadcast:   make(chan *broadcastMessage, 256),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for now
			},
		},
	}
}

// Run starts the hub's main loop.
func (h *WebSocketHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return

		case reg := <-h.register:
			h.mu.Lock()
			if _, ok := h.connections[reg.benchID]; !ok {
				h.connections[reg.benchID] = make(map[*WebSocketConn]bool)
			}
			h.connections[reg.benchID][reg.conn] = true
			h.mu.Unlock()
			log.Printf("WebSocket client connected for benchmark %s", reg.benchID)

		case conn := <-h.unregister:
			h.mu.Lock()
			if conns, ok := h.connections[conn.benchID]; ok {
				if _, ok := conns[conn]; ok {
					delete(conns, conn)
					close(conn.send)
					if len(conns) == 0 {
						delete(h.connections, conn.benchID)
					}
				}
			}
			h.mu.Unlock()
			log.Printf("WebSocket client disconnected for benchmark %s", conn.benchID)

		case msg := <-h.broadcast:
			h.mu.RLock()
			if conns, ok := h.connections[msg.benchID]; ok {
				for conn := range conns {
					select {
					case conn.send <- msg.message:
					default:
						// Connection is slow, close it
						close(conn.send)
						delete(conns, conn)
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast sends a message to all connections for a benchmark.
func (h *WebSocketHub) Broadcast(benchID string, msg *WebSocketMessage) {
	msg.Timestamp = time.Now()
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal WebSocket message: %v", err)
		return
	}

	h.broadcast <- &broadcastMessage{
		benchID: benchID,
		message: data,
	}
}

// BroadcastProgress sends progress update to all connections for a benchmark.
func (h *WebSocketHub) BroadcastProgress(benchID string, progress *ProgressData) {
	h.Broadcast(benchID, &WebSocketMessage{
		Type:    "benchmark.progress",
		BenchID: benchID,
		Data:    progress,
	})
}

// BroadcastStarted sends benchmark started event.
func (h *WebSocketHub) BroadcastStarted(benchID string, config interface{}) {
	h.Broadcast(benchID, &WebSocketMessage{
		Type:    "benchmark.started",
		BenchID: benchID,
		Data:    config,
	})
}

// BroadcastCompleted sends benchmark completed event.
func (h *WebSocketHub) BroadcastCompleted(benchID string, results interface{}) {
	h.Broadcast(benchID, &WebSocketMessage{
		Type:    "benchmark.completed",
		BenchID: benchID,
		Data:    results,
	})
}

// BroadcastFailed sends benchmark failed event.
func (h *WebSocketHub) BroadcastFailed(benchID string, err error) {
	h.Broadcast(benchID, &WebSocketMessage{
		Type:    "benchmark.failed",
		BenchID: benchID,
		Data: map[string]string{
			"error": err.Error(),
		},
	})
}

// HandleWebSocket handles WebSocket upgrade requests.
func (h *WebSocketHub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	benchID := r.URL.Query().Get("bench_id")
	if benchID == "" {
		benchID = "*" // Subscribe to all benchmarks
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	wsConn := &WebSocketConn{
		hub:      h,
		conn:     conn,
		send:     make(chan []byte, 256),
		benchID:  benchID,
		clientID: r.RemoteAddr,
	}

	h.register <- &connRegistration{
		conn:    wsConn,
		benchID: benchID,
	}

	// Start read and write goroutines
	go wsConn.writePump()
	go wsConn.readPump()
}

// ConnectionCount returns the number of active connections for a benchmark.
func (h *WebSocketHub) ConnectionCount(benchID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if conns, ok := h.connections[benchID]; ok {
		return len(conns)
	}
	return 0
}

// readPump pumps messages from the WebSocket connection to the hub.
func (c *WebSocketConn) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(1024)
	c.conn.SetReadDeadline(time.Now().Add(90 * time.Second)) // Extended timeout
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// Only log unexpected errors (not normal closes)
			if websocket.IsUnexpectedCloseError(err, 
				websocket.CloseGoingAway, 
				websocket.CloseAbnormalClosure,
				websocket.CloseNormalClosure,
				websocket.CloseNoStatusReceived) {
				log.Printf("WebSocket read error: %v", err)
			}
			break
		}

		// Reset read deadline on any message (including pings)
		c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		// Handle incoming messages (e.g., subscribe/unsubscribe)
		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the WebSocket connection.
func (c *WebSocketConn) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				// Channel closed, send close message gracefully
				c.conn.WriteMessage(websocket.CloseMessage, 
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				// Write error - connection is likely dead, exit silently
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				// Ping failed - connection is dead, exit silently
				return
			}
		}
	}
}

// handleMessage handles incoming WebSocket messages.
func (c *WebSocketConn) handleMessage(message []byte) {
	var msg struct {
		Type    string `json:"type"`
		BenchID string `json:"bench_id"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		return
	}

	switch msg.Type {
	case "subscribe":
		// Update subscription
		if msg.BenchID != "" {
			c.hub.unregister <- c
			c.benchID = msg.BenchID
			c.hub.register <- &connRegistration{
				conn:    c,
				benchID: c.benchID,
			}
		}

	case "ping":
		// Respond with pong
		response, _ := json.Marshal(map[string]interface{}{
			"type":      "pong",
			"timestamp": time.Now(),
		})
		c.send <- response
	}
}
