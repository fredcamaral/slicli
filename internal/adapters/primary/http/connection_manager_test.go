package http

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

func TestConnectionManager(t *testing.T) {
	t.Run("create new connection manager", func(t *testing.T) {
		cm := NewConnectionManager()
		assert.NotNil(t, cm)
		assert.NotNil(t, cm.connections)
		assert.NotNil(t, cm.broadcast)
		assert.NotNil(t, cm.register)
		assert.NotNil(t, cm.unregister)
	})

	t.Run("register and unregister connection", func(t *testing.T) {
		cm := NewConnectionManager()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start the connection manager
		go cm.Run(ctx)

		// Create a connection
		conn := &Connection{
			ID:   "test-conn",
			Send: make(chan ports.UpdateEvent, 1),
		}
		cm.RegisterConnection(conn)

		// Give it time to process
		time.Sleep(10 * time.Millisecond)

		// Check connection was registered
		cm.mu.RLock()
		assert.Len(t, cm.connections, 1)
		cm.mu.RUnlock()

		// Unregister connection
		cm.Unregister("test-conn")
		time.Sleep(10 * time.Millisecond)

		// Check connection was removed
		cm.mu.RLock()
		assert.Len(t, cm.connections, 0)
		cm.mu.RUnlock()
	})

	t.Run("broadcast to connections", func(t *testing.T) {
		cm := NewConnectionManager()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start the connection manager
		go cm.Run(ctx)

		// Register multiple connections
		receivers := make([]chan ports.UpdateEvent, 3)
		for i := 0; i < 3; i++ {
			receivers[i] = make(chan ports.UpdateEvent, 1)
			conn := &Connection{
				ID:   string(rune('a' + i)),
				Send: receivers[i],
			}
			cm.RegisterConnection(conn)
		}

		// Give it time to process registrations
		time.Sleep(10 * time.Millisecond)

		// Broadcast an event
		event := ports.UpdateEvent{
			Type:      "test",
			Timestamp: time.Now(),
		}
		cm.Broadcast(event)

		// Check all connections received the event
		for i, receiver := range receivers {
			select {
			case received := <-receiver:
				assert.Equal(t, event.Type, received.Type)
			case <-time.After(100 * time.Millisecond):
				t.Errorf("Connection %d did not receive event", i)
			}
		}
	})

	t.Run("close all connections", func(t *testing.T) {
		cm := NewConnectionManager()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start the connection manager
		go cm.Run(ctx)

		// Register connections
		for i := 0; i < 5; i++ {
			conn := &Connection{
				ID:   string(rune('a' + i)),
				Send: make(chan ports.UpdateEvent, 1),
			}
			cm.RegisterConnection(conn)
		}

		// Give it time to process
		time.Sleep(10 * time.Millisecond)

		// Check connections were registered
		cm.mu.RLock()
		assert.Len(t, cm.connections, 5)
		cm.mu.RUnlock()

		// Close all connections
		cm.CloseAll()

		// Check all connections were removed
		cm.mu.RLock()
		assert.Len(t, cm.connections, 0)
		cm.mu.RUnlock()
	})

	t.Run("concurrent operations", func(t *testing.T) {
		cm := NewConnectionManager()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Start the connection manager
		go cm.Run(ctx)

		var wg sync.WaitGroup
		numGoroutines := 10
		numOperations := 100

		// Perform concurrent operations
		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				for j := 0; j < numOperations; j++ {
					connID := string(rune('a'+id)) + string(rune('0'+j%10))

					// Register
					conn := &Connection{
						ID:   connID,
						Send: make(chan ports.UpdateEvent, 1),
					}
					cm.RegisterConnection(conn)

					// Broadcast
					event := ports.UpdateEvent{
						Type:      "test",
						Timestamp: time.Now(),
					}
					cm.Broadcast(event)

					// Unregister
					cm.Unregister(connID)
				}
			}(i)
		}

		wg.Wait()

		// Give time for all operations to complete
		time.Sleep(50 * time.Millisecond)

		// All connections should be cleaned up
		cm.mu.RLock()
		assert.Len(t, cm.connections, 0)
		cm.mu.RUnlock()
	})
}

func TestConnectionManagerShutdown(t *testing.T) {
	cm := NewConnectionManager()
	ctx, cancel := context.WithCancel(context.Background())

	// Start the connection manager
	go cm.Run(ctx)

	// Register a connection
	conn := &Connection{
		ID:   "test",
		Send: make(chan ports.UpdateEvent, 1),
	}
	cm.RegisterConnection(conn)

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	// Cancel context to shutdown
	cancel()

	// Try to broadcast after shutdown
	event := ports.UpdateEvent{
		Type:      "test",
		Timestamp: time.Now(),
	}

	// Should not hang
	done := make(chan bool)
	go func() {
		cm.Broadcast(event)
		done <- true
	}()

	select {
	case <-done:
		// Good, didn't hang
	case <-time.After(100 * time.Millisecond):
		t.Error("Broadcast hung after shutdown")
	}
}
