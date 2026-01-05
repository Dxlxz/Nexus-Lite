// Package main provides WebSocket functionality for real-time dashboard updates.
// This module manages WebSocket connections and broadcasts transaction data,
// metrics, and bank balances to connected dashboard clients.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket upgrader with configuration for connection upgrades
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		log.Printf("[WebSocket] CheckOrigin: %s %s", r.Method, r.URL.Path)
		return true // Allow all origins for development
	},
}

// WebSocketClient represents a connected WebSocket client.
// Each client has its own send channel for asynchronous message delivery.
type WebSocketClient struct {
	conn     *websocket.Conn // WebSocket connection
	send     chan []byte     // Channel for outgoing messages
	hub      *WebSocketHub   // Reference to the hub for cleanup
	mu       sync.Mutex      // Protects connection state
	isClosed bool            // Tracks if connection is closed
}

// WebSocketHub manages WebSocket connections and message broadcasting.
// It maintains a registry of active clients and handles connection lifecycle.
type WebSocketHub struct {
	clients    map[*WebSocketClient]bool // Active client connections
	register   chan *WebSocketClient     // Channel for new client registrations
	unregister chan *WebSocketClient     // Channel for client disconnections
	broadcast  chan []byte               // Channel for broadcast messages (buffered)
	mu         sync.RWMutex              // Protects client map access
}

// NewWebSocketHub creates a new WebSocket hub with initialized channels.
// Returns a pointer to the WebSocketHub ready for client management.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*WebSocketClient]bool),
		register:   make(chan *WebSocketClient),
		unregister: make(chan *WebSocketClient),
		broadcast:  make(chan []byte, 256),
	}
}

// Run starts the WebSocket hub event loop.
// It handles client registration, unregistration, and message broadcasting.
func (h *WebSocketHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WebSocket] Client connected. Total clients: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WebSocket] Client disconnected. Total clients: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client send buffer is full, close connection
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastMessage broadcasts a message to all connected clients.
// It marshals the message to JSON and sends it through the broadcast channel.
func (h *WebSocketHub) BroadcastMessage(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return err
	}

	select {
	case h.broadcast <- data:
	default:
		log.Println("[WebSocket] Broadcast channel full, message dropped")
	}

	return nil
}

// writePump pumps messages from the hub to the websocket connection.
// It handles ping/pong for connection health and graceful disconnection.
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.mu.Lock()
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				c.mu.Unlock()
				return
			}

			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()

		case <-ticker.C:
			c.mu.Lock()
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}
}

// readPump pumps messages from the websocket connection to the hub.
// It handles pong responses for connection keepalive and manages disconnections.
func (c *WebSocketClient) readPump() {
	defer func() {
		log.Printf("[WebSocket] Closing connection for %s", c.conn.RemoteAddr())
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[WebSocket] Read error from %s: %v", c.conn.RemoteAddr(), err)
			break
		}
	}
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// TransactionMessage represents a transaction update
type TransactionMessage struct {
	ID            string    `json:"id"`
	MsgID         string    `json:"msgId"`
	Source        string    `json:"source"`
	SourceCountry string    `json:"sourceCountry"`
	Destination   string    `json:"destination"`
	DestCountry   string    `json:"destCountry"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	Status        string    `json:"status"`
	ErrorCode     string    `json:"errorCode,omitempty"`
	ErrorMsg      string    `json:"errorMsg,omitempty"`
	Timestamp     time.Time `json:"timestamp"`
	XML           string    `json:"xml"`
	Latency       int64     `json:"latency"`
}

// MetricsMessage represents metrics update
type MetricsMessage struct {
	TotalProcessed    int64   `json:"totalProcessed"`
	MessagesPerSec    float64 `json:"messagesPerSecond"`
	SuccessRate       float64 `json:"successRate"`
	ActiveConnections int     `json:"activeConnections"`
}

// StatusMessage represents status update
type StatusMessage struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

// BankBalanceMessage represents a bank balance update
type BankBalanceMessage struct {
	BankName string  `json:"bankName"`
	BIC      string  `json:"bic"`
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

// ReadinessStatus represents readiness check response
type ReadinessStatus struct {
	Ready          bool   `json:"ready"`
	KafkaReady     bool   `json:"kafka_ready"`
	LiquidityReady bool   `json:"liquidity_ready"`
	Message        string `json:"message,omitempty"`
}

// Global WebSocket hub
var wsHub *WebSocketHub

// StartWebSocketServer starts the WebSocket server with HTTP endpoints.
// It initializes the hub, sets up routes for WebSocket and health checks,
// and starts the HTTP server in a background goroutine.
func StartWebSocketServer(addr string) *WebSocketHub {
	wsHub = NewWebSocketHub()
	go wsHub.Run()

	http.HandleFunc("/ws", handleWebSocket)
	http.HandleFunc("/health", handleHealth)
	http.HandleFunc("/ready", handleReady)

	go func() {
		log.Printf("[WebSocket] Server starting on %s", addr)
		if err := http.ListenAndServe(addr, nil); err != nil {
			log.Fatalf("[WebSocket] Server error: %v", err)
		}
	}()

	return wsHub
}

// handleWebSocket handles WebSocket connections.
// It upgrades HTTP connections to WebSocket and registers clients with the hub.
func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("[WebSocket] Handling connection from %s", r.RemoteAddr)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WebSocket] Upgrade error: %v", err)
		return
	}
	log.Printf("[WebSocket] Upgrade successful for %s", r.RemoteAddr)

	client := &WebSocketClient{
		conn: conn,
		send: make(chan []byte, 256),
		hub:  wsHub,
	}

	wsHub.register <- client

	// Send initial status
	statusMsg := WebSocketMessage{
		Type: "status",
		Data: StatusMessage{
			Status:  "connected",
			Message: "Connected to Nexus-Lite WebSocket",
		},
	}
	statusData, _ := json.Marshal(statusMsg)
	client.send <- statusData

	// Start pumps
	go client.writePump()
	client.readPump()
}

// handleHealth handles health check requests
func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"clients": len(wsHub.clients),
	})
}

// handleReady handles readiness check requests
func handleReady(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	kafkaOk := kafkaReady == 1
	liquidityOk := liquidityReady == 1
	ready := kafkaOk && liquidityOk

	status := ReadinessStatus{
		Ready:          ready,
		KafkaReady:     kafkaOk,
		LiquidityReady: liquidityOk,
	}

	if !ready {
		status.Message = "Waiting for dependencies"
	}

	if ready {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusServiceUnavailable)
	}

	json.NewEncoder(w).Encode(status)
}

// BroadcastTransaction broadcasts a transaction to all connected clients.
// It wraps the transaction in a WebSocket message and sends it through the hub.
func BroadcastTransaction(tx TransactionMessage) error {
	msg := WebSocketMessage{
		Type: "transaction",
		Data: tx,
	}
	return wsHub.BroadcastMessage(msg)
}

// BroadcastMetrics broadcasts metrics to all connected clients
func BroadcastMetrics(metrics MetricsMessage) error {
	msg := WebSocketMessage{
		Type: "metrics",
		Data: metrics,
	}
	return wsHub.BroadcastMessage(msg)
}

// BroadcastBalances broadcasts bank balances to all connected clients
// BroadcastBalances broadcasts bank balance updates to all connected clients.
// It wraps the balances in a WebSocket message for dashboard display.
func BroadcastBalances(balances []BankBalanceMessage) error {
	msg := WebSocketMessage{
		Type: "balances",
		Data: balances,
	}
	return wsHub.BroadcastMessage(msg)
}
