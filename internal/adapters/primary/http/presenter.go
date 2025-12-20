package http

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
	"github.com/fredcamaral/slicli/internal/infrastructure/logging"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

// PresenterHandler handles presenter mode requests
type PresenterHandler struct {
	syncService ports.PresentationSync
	upgrader    websocket.Upgrader
	logger      *logging.Logger
	config      *entities.ServerConfig
}

// NewPresenterHandler creates a new presenter handler
// Deprecated: Use NewPresenterHandlerWithConfig instead for proper origin validation
func NewPresenterHandler(sync ports.PresentationSync) *PresenterHandler {
	h := &PresenterHandler{
		syncService: sync,
		logger:      NewHTTPLogger("presenter", false),
		config:      nil, // No config means development mode (allows all origins)
	}
	h.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return h.isValidOrigin(r)
		},
	}
	return h
}

// NewPresenterHandlerWithConfig creates a new presenter handler with server config for origin validation
func NewPresenterHandlerWithConfig(sync ports.PresentationSync, config *entities.ServerConfig) *PresenterHandler {
	h := &PresenterHandler{
		syncService: sync,
		logger:      NewHTTPLogger("presenter", false),
		config:      config,
	}
	h.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return h.isValidOrigin(r)
		},
	}
	return h
}

// NewPresenterHandlerWithLogging creates a new presenter handler with logging configuration
// Deprecated: Use NewPresenterHandlerFull instead for proper origin validation
func NewPresenterHandlerWithLogging(sync ports.PresentationSync, loggingConfig *entities.LoggingConfig) *PresenterHandler {
	return NewPresenterHandlerFull(sync, nil, loggingConfig)
}

// NewPresenterHandlerFull creates a new presenter handler with full configuration
func NewPresenterHandlerFull(sync ports.PresentationSync, config *entities.ServerConfig, loggingConfig *entities.LoggingConfig) *PresenterHandler {
	level := entities.LogLevelInfo
	verbose := false

	if loggingConfig != nil {
		level = loggingConfig.GetLevel()
		verbose = loggingConfig.Verbose
	}

	h := &PresenterHandler{
		syncService: sync,
		logger:      NewHTTPLoggerWithLevel("presenter", verbose, level),
		config:      config,
	}
	h.upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return h.isValidOrigin(r)
		},
	}
	return h
}

// isValidOrigin validates WebSocket connection origins
func (h *PresenterHandler) isValidOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")

	// No origin header - likely same-origin request
	if origin == "" {
		return true
	}

	// No config means development mode - allow all origins
	if h.config == nil {
		return true
	}

	// Check configured CORS origins
	for _, allowed := range h.config.GetCORSOrigins() {
		// Wildcard origin only allowed in development mode
		if allowed == "*" && h.config.IsDevelopment() {
			return true
		}
		if origin == allowed {
			return true
		}
	}

	// In development mode, be more permissive
	return h.config.IsDevelopment()
}

// RegisterRoutes registers presenter mode routes
func (h *PresenterHandler) RegisterRoutes(router *mux.Router) {
	// Presenter view
	router.HandleFunc("/presenter", h.HandlePresenterView).Methods("GET")

	// WebSocket for real-time sync
	router.HandleFunc("/presenter/ws", h.HandleWebSocket).Methods("GET")

	// Navigation API
	router.HandleFunc("/presenter/navigate", h.HandleNavigation).Methods("POST")

	// Timer API
	router.HandleFunc("/presenter/timer", h.HandleTimer).Methods("POST")

	// State API
	router.HandleFunc("/presenter/state", h.HandleState).Methods("GET")
}

// HandlePresenterView serves the presenter mode interface
func (h *PresenterHandler) HandlePresenterView(w http.ResponseWriter, r *http.Request) {
	state := h.syncService.GetState()

	// For now, return JSON response (template will be added later)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		http.Error(w, "Failed to encode presenter state", http.StatusInternalServerError)
		return
	}
}

// HandleWebSocket handles WebSocket connections for real-time sync
func (h *PresenterHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("WebSocket upgrade failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()

	clientID := generateClientID()
	events := h.syncService.Subscribe(clientID)
	defer h.syncService.Unsubscribe(clientID)

	// Send initial state
	initialState := h.syncService.GetState()
	stateEvent := entities.NewSyncEvent("state", map[string]interface{}{
		"state": initialState,
	})

	if err := conn.WriteJSON(stateEvent); err != nil {
		h.logger.Error("Failed to send initial state: %v", err)
		return
	}

	// Handle incoming messages in a separate goroutine
	go h.handleIncoming(conn, clientID)

	// Send outgoing events
	for event := range events {
		if err := conn.WriteJSON(event); err != nil {
			h.logger.Error("Failed to send event to client %s: %v", clientID, err)
			return
		}
	}
}

// handleIncoming processes incoming WebSocket messages
func (h *PresenterHandler) handleIncoming(conn *websocket.Conn, clientID string) {
	for {
		var event entities.SyncEvent
		if err := conn.ReadJSON(&event); err != nil {
			h.logger.Warn("Failed to read message from client %s: %v", clientID, err)
			return
		}

		// Broadcast the event to other clients
		if err := h.syncService.Broadcast(event); err != nil {
			h.logger.Error("Failed to broadcast event: %v", err)
		}
	}
}

// HandleNavigation handles navigation requests
func (h *PresenterHandler) HandleNavigation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
		Slide  int    `json:"slide,omitempty"`
	}

	if err := decodeJSONBody(w, r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create navigation event
	eventData := map[string]interface{}{
		"action": req.Action,
	}

	if req.Slide >= 0 {
		eventData["slide"] = float64(req.Slide)
	}

	event := entities.NewSyncEvent("navigation", eventData)

	if err := h.syncService.Broadcast(event); err != nil {
		http.Error(w, "Failed to broadcast navigation event", http.StatusInternalServerError)
		return
	}

	// Return updated state
	state := h.syncService.GetState()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		http.Error(w, "Failed to encode state", http.StatusInternalServerError)
		return
	}
}

// HandleTimer handles timer control requests
func (h *PresenterHandler) HandleTimer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action string `json:"action"`
	}

	if err := decodeJSONBody(w, r, &req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate action
	validActions := map[string]bool{
		"pause":  true,
		"resume": true,
		"reset":  true,
	}

	if !validActions[req.Action] {
		http.Error(w, "Invalid timer action", http.StatusBadRequest)
		return
	}

	// Create timer event
	event := entities.NewSyncEvent("timer", map[string]interface{}{
		"action": req.Action,
	})

	if err := h.syncService.Broadcast(event); err != nil {
		http.Error(w, "Failed to broadcast timer event", http.StatusInternalServerError)
		return
	}

	// Return updated state
	state := h.syncService.GetState()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		http.Error(w, "Failed to encode state", http.StatusInternalServerError)
		return
	}
}

// HandleState returns the current presenter state
func (h *PresenterHandler) HandleState(w http.ResponseWriter, r *http.Request) {
	state := h.syncService.GetState()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(state); err != nil {
		http.Error(w, "Failed to encode state", http.StatusInternalServerError)
		return
	}
}

// generateClientID generates a unique client ID.
func generateClientID() string {
	bytes := make([]byte, 8)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to timestamp-based ID using current time
		return fmt.Sprintf("client-%d", time.Now().UnixNano())
	}
	return "client-" + hex.EncodeToString(bytes)
}
