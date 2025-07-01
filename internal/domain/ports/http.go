package ports

import (
	"context"
	"time"
)

// HTTPServer defines the interface for the HTTP server
type HTTPServer interface {
	Start(ctx context.Context, port int, host string) error
	Stop(ctx context.Context) error
	NotifyClients(event UpdateEvent) error
	IsRunning() bool
}

// UpdateEvent represents an event sent to WebSocket clients
type UpdateEvent struct {
	Type      string      `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// UpdateEventType constants
const (
	EventTypeReload         = "reload"
	EventTypeFileChange     = "file_change"
	EventTypeError          = "error"
	EventTypePresenterState = "presenter_state"
	EventTypeNavigation     = "navigation"
	EventTypeTimer          = "timer"
	EventTypeNotesUpdate    = "notes_update"
)
