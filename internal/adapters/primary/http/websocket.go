package http

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// createUpgrader creates a WebSocket upgrader with proper origin validation
func (s *Server) createUpgrader() websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return s.isValidOrigin(r)
		},
	}
}

// ClientMode represents the type of WebSocket client
type ClientMode string

const (
	ClientModeAudience  ClientMode = "audience"
	ClientModePresenter ClientMode = "presenter"
)

// WebSocketClient represents a WebSocket client connection
type WebSocketClient struct {
	id      string
	conn    *websocket.Conn
	send    chan ports.UpdateEvent
	manager *ConnectionManager
	mode    ClientMode
	logger  *HTTPLogger
}

// ClientMessage represents a message received from the client
type ClientMessage struct {
	Type string                 `json:"type"`
	Data map[string]interface{} `json:"data"`
}

// handleWebSocket handles WebSocket upgrade requests
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	upgrader := s.createUpgrader()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("WebSocket upgrade failed: %v", err)
		return
	}

	// Determine client mode from query parameter
	mode := ClientModeAudience
	if r.URL.Query().Get("mode") == "presenter" {
		mode = ClientModePresenter
	}

	client := &WebSocketClient{
		id:      uuid.New().String(),
		conn:    conn,
		send:    make(chan ports.UpdateEvent, 256),
		manager: s.connMgr,
		mode:    mode,
		logger:  s.logger,
	}

	// Register the client with connection manager
	connInfo := &Connection{
		ID:   client.id,
		Send: client.send,
	}
	s.connMgr.register <- connInfo

	// Start goroutines for reading and writing
	go client.writePump()
	go client.readPump()

	// Send initial connection event
	event := ports.UpdateEvent{
		Type:      "connected",
		Timestamp: time.Now(),
		Data: map[string]string{
			"message": "Connected to slicli server",
			"version": "1.0.0",
		},
	}

	select {
	case client.send <- event:
	default:
		// Client's send channel is full
	}
}

// readPump pumps messages from the WebSocket connection
func (c *WebSocketClient) readPump() {
	defer func() {
		c.manager.Unregister(c.id)
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		// Read message from browser
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.logger.Error("WebSocket connection error: %v", err)
			}
			break
		}

		// Handle presenter messages
		if c.mode == ClientModePresenter {
			var clientMsg ClientMessage
			if err := json.Unmarshal(message, &clientMsg); err != nil {
				c.logger.Error("Failed to parse client message: %v", err)
				continue
			}

			// Handle presenter commands
			c.handlePresenterCommand(clientMsg)
		} else {
			// For audience clients, just log the message
			c.logger.Debug("Received message from audience client %s: %s", c.id, message)
		}
	}
}

// writePump pumps messages to the WebSocket connection
func (c *WebSocketClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case event, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The channel has been closed
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Write the event as JSON
			if err := c.conn.WriteJSON(event); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handlePresenterCommand handles commands from presenter clients
func (c *WebSocketClient) handlePresenterCommand(msg ClientMessage) {
	// Get the server instance to access the sync service
	// This is a bit of a hack - ideally the client would have access to services
	// For now, we'll create a sync event and let the server handle it

	syncEvent := entities.NewSyncEvent(msg.Type, msg.Data)

	// Convert to UpdateEvent and broadcast to all clients
	updateEvent := ports.UpdateEvent{
		Type:      msg.Type,
		Timestamp: syncEvent.Timestamp,
		Data:      msg.Data,
	}

	// Send to all clients through the connection manager
	c.manager.Broadcast(updateEvent)

	c.logger.Debug("Handled presenter command from client %s: %s", c.id, msg.Type)
}

// BroadcastReload sends a reload event to all connected clients
func (s *Server) BroadcastReload() {
	event := ports.UpdateEvent{
		Type:      ports.EventTypeReload,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"message": "Presentation updated",
		},
	}
	_ = s.NotifyClients(event)
}

// BroadcastFileChange sends a file change event to all connected clients
func (s *Server) BroadcastFileChange(filename string) {
	event := ports.UpdateEvent{
		Type:      ports.EventTypeFileChange,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"file":    filename,
			"message": "File changed",
		},
	}
	_ = s.NotifyClients(event)
}

// isValidOrigin validates WebSocket connection origins based on environment
func (s *Server) isValidOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// Allow empty origin (same-origin requests)
	if origin == "" {
		return true
	}

	// Parse origin URL
	originURL, err := url.Parse(origin)
	if err != nil {
		s.logger.Warn("WebSocket connection rejected: invalid origin URL", "origin", origin, "error", err)
		return false
	}

	// Development mode: allow localhost and LAN addresses
	if s.config.IsDevelopment() {
		return s.isDevelopmentOrigin(originURL)
	}

	// Production mode: strict whitelist validation
	return s.isProductionOrigin(originURL)
}

// isDevelopmentOrigin validates origins for development environment
func (s *Server) isDevelopmentOrigin(originURL *url.URL) bool {
	hostname := originURL.Hostname()

	// Allow localhost, 127.0.0.1, and LAN addresses for development
	allowedHosts := []string{
		"localhost",
		"127.0.0.1",
		"0.0.0.0",
	}

	for _, allowed := range allowedHosts {
		if hostname == allowed {
			return true
		}
	}

	// Allow private network ranges (192.168.x.x, 10.x.x.x, 172.16-31.x.x)
	if strings.HasPrefix(hostname, "192.168.") ||
		strings.HasPrefix(hostname, "10.") ||
		s.isPrivateClassB(hostname) {
		return true
	}

	return false
}

// isProductionOrigin validates origins for production environment
func (s *Server) isProductionOrigin(originURL *url.URL) bool {
	// Production: use configured CORS origins
	for _, allowedOrigin := range s.config.GetCORSOrigins() {
		if originURL.String() == allowedOrigin {
			return true
		}

		// Support wildcard subdomains (*.example.com)
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(originURL.Hostname(), domain) {
				return true
			}
		}
	}

	s.logger.Warn("WebSocket connection rejected: origin not in whitelist",
		"origin", originURL.String(),
		"allowed_origins", s.config.GetCORSOrigins())
	return false
}

// isPrivateClassB checks for 172.16.0.0 to 172.31.255.255 range
func (s *Server) isPrivateClassB(hostname string) bool {
	if !strings.HasPrefix(hostname, "172.") {
		return false
	}

	parts := strings.Split(hostname, ".")
	if len(parts) < 2 {
		return false
	}

	// Check if second octet is between 16-31
	secondOctet := parts[1]
	switch secondOctet {
	case "16", "17", "18", "19", "20", "21", "22", "23", "24", "25", "26", "27", "28", "29", "30", "31":
		return true
	default:
		return false
	}
}
