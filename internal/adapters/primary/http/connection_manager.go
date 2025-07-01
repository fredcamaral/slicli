package http

import (
	"context"
	"sync"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// Connection represents a WebSocket connection
type Connection struct {
	ID   string
	Send chan ports.UpdateEvent
}

// ConnectionManager manages WebSocket connections
type ConnectionManager struct {
	connections map[string]*Connection
	broadcast   chan ports.UpdateEvent
	register    chan *Connection
	unregister  chan string
	mu          sync.RWMutex
	done        chan struct{}
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: make(map[string]*Connection),
		broadcast:   make(chan ports.UpdateEvent, 256),
		register:    make(chan *Connection),
		unregister:  make(chan string),
		done:        make(chan struct{}),
	}
}

// Run starts the connection manager main loop
func (cm *ConnectionManager) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			close(cm.done)
			return
		case conn := <-cm.register:
			cm.mu.Lock()
			cm.connections[conn.ID] = conn
			cm.mu.Unlock()

		case id := <-cm.unregister:
			cm.mu.Lock()
			if conn, ok := cm.connections[id]; ok {
				delete(cm.connections, id)
				close(conn.Send)
			}
			cm.mu.Unlock()

		case event := <-cm.broadcast:
			cm.mu.RLock()
			for _, conn := range cm.connections {
				select {
				case conn.Send <- event:
				default:
					// Client too slow, close connection
					close(conn.Send)
					delete(cm.connections, conn.ID)
				}
			}
			cm.mu.RUnlock()
		}
	}
}

// RegisterConnection adds a new connection directly
func (cm *ConnectionManager) RegisterConnection(conn *Connection) {
	cm.register <- conn
}

// Unregister removes a connection
func (cm *ConnectionManager) Unregister(connID string) {
	cm.unregister <- connID
}

// Broadcast sends an event to all connections
func (cm *ConnectionManager) Broadcast(event ports.UpdateEvent) {
	select {
	case cm.broadcast <- event:
	case <-cm.done:
		// Manager is shutting down
	}
}

// CloseAll closes all connections
func (cm *ConnectionManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, conn := range cm.connections {
		close(conn.Send)
		delete(cm.connections, id)
	}
}
