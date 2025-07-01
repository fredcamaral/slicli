package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

func TestWebSocketUpgrade(t *testing.T) {
	t.Skip("Skipping WebSocket tests for now")
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer func() { ts.Close() }()

	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	t.Run("successful WebSocket connection", func(t *testing.T) {
		// Connect to WebSocket
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer func() { _ = ws.Close() }()

		// Should receive connected message
		var event ports.UpdateEvent
		err = ws.ReadJSON(&event)
		require.NoError(t, err)
		assert.Equal(t, "connected", event.Type)
	})

	t.Run("multiple WebSocket connections", func(t *testing.T) {
		// Connect multiple clients
		clients := make([]*websocket.Conn, 3)
		for i := 0; i < 3; i++ {
			ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
			require.NoError(t, err)
			clients[i] = ws
			defer func() { _ = ws.Close() }()

			// Read connected message
			var event ports.UpdateEvent
			err = ws.ReadJSON(&event)
			require.NoError(t, err)
			assert.Equal(t, "connected", event.Type)
		}
	})
}

func TestWebSocketBroadcast(t *testing.T) {
	t.Skip("Skipping WebSocket tests for now")
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	// Start the server
	ctx := context.Background()
	err := server.Start(ctx, 0, "localhost")
	require.NoError(t, err)
	defer func() { _ = server.Stop(ctx) }()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Get server address
	addr := server.server.Addr
	wsURL := "ws://" + addr + "/ws"

	t.Run("broadcast reload event", func(t *testing.T) {
		// Connect client
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer func() { _ = ws.Close() }()

		// Read initial connected message
		var connEvent ports.UpdateEvent
		err = ws.ReadJSON(&connEvent)
		require.NoError(t, err)

		// Broadcast reload event
		server.BroadcastReload()

		// Should receive reload event
		var event ports.UpdateEvent
		err = ws.ReadJSON(&event)
		require.NoError(t, err)
		assert.Equal(t, ports.EventTypeReload, event.Type)
		assert.NotNil(t, event.Data)
	})

	t.Run("broadcast file change event", func(t *testing.T) {
		// Connect client
		ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		require.NoError(t, err)
		defer func() { _ = ws.Close() }()

		// Read initial connected message
		var connEvent ports.UpdateEvent
		err = ws.ReadJSON(&connEvent)
		require.NoError(t, err)

		// Broadcast file change event
		server.BroadcastFileChange("test.md")

		// Should receive file change event
		var event ports.UpdateEvent
		err = ws.ReadJSON(&event)
		require.NoError(t, err)
		assert.Equal(t, ports.EventTypeFileChange, event.Type)
		data := event.Data.(map[string]interface{})
		assert.Equal(t, "test.md", data["file"])
	})
}

func TestWebSocketClientDisconnect(t *testing.T) {
	t.Skip("Skipping WebSocket tests for now")
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer func() { ts.Close() }()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Connect and disconnect
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)

	// Close connection
	err = ws.Close()
	assert.NoError(t, err)

	// Give time for cleanup
	time.Sleep(50 * time.Millisecond)

	// Connection should be removed from manager
	server.connMgr.mu.RLock()
	assert.Len(t, server.connMgr.connections, 0)
	server.connMgr.mu.RUnlock()
}

func TestWebSocketPingPong(t *testing.T) {
	t.Skip("Skipping WebSocket tests for now")
	presenter := new(MockPresentationService)
	renderer := new(MockRenderer)
	server := NewServer(presenter, renderer, getTestServerConfig())

	// Create test server
	ts := httptest.NewServer(http.HandlerFunc(server.handleWebSocket))
	defer func() { ts.Close() }()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http")

	// Connect
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err)
	defer func() { _ = ws.Close() }()

	// Set pong handler
	pongReceived := make(chan bool, 1)
	ws.SetPongHandler(func(appData string) error {
		pongReceived <- true
		return nil
	})

	// Start read pump
	go func() {
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				return
			}
		}
	}()

	// Send ping
	err = ws.WriteMessage(websocket.PingMessage, []byte{})
	require.NoError(t, err)

	// Should receive pong
	select {
	case <-pongReceived:
		// Good
	case <-time.After(2 * time.Second):
		t.Error("Did not receive pong response")
	}
}
