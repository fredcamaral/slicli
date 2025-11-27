package http

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// Connection represents a WebSocket connection
type Connection struct {
	ID        string
	Send      chan ports.UpdateEvent
	closeOnce sync.Once
}

// Close safely closes the connection's send channel exactly once.
// It is safe to call this method multiple times.
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		close(c.Send)
	})
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
				conn.Close()
			}
			cm.mu.Unlock()

		case event := <-cm.broadcast:
			var slowClients []string
			cm.mu.RLock()
			for _, conn := range cm.connections {
				select {
				case conn.Send <- event:
				default:
					// Client too slow, mark for removal
					slowClients = append(slowClients, conn.ID)
				}
			}
			cm.mu.RUnlock()

			// Remove slow clients with write lock
			if len(slowClients) > 0 {
				cm.mu.Lock()
				for _, id := range slowClients {
					if conn, ok := cm.connections[id]; ok {
						conn.Close()
						delete(cm.connections, id)
					}
				}
				cm.mu.Unlock()
			}
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

// broadcastTimeout is the maximum time to wait when sending a broadcast event.
// If the broadcast channel is full for longer than this duration, the event is dropped.
const broadcastTimeout = 100 * time.Millisecond

// Broadcast sends an event to all connections.
// If the broadcast channel is full and cannot accept the event within broadcastTimeout,
// the event is dropped and a warning is logged.
func (cm *ConnectionManager) Broadcast(event ports.UpdateEvent) {
	select {
	case cm.broadcast <- event:
	case <-cm.done:
		// Manager is shutting down
	case <-time.After(broadcastTimeout):
		log.Printf("WARNING: broadcast event dropped due to backpressure (type: %s)", event.Type)
	}
}

// CloseAll closes all connections
func (cm *ConnectionManager) CloseAll() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for id, conn := range cm.connections {
		conn.Close()
		delete(cm.connections, id)
	}
}
