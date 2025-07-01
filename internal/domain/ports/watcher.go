package ports

import (
	"context"
	"time"
)

// FileWatcher defines the interface for watching file changes
type FileWatcher interface {
	// Watch starts watching a file for changes
	Watch(ctx context.Context, path string) (<-chan FileChangeEvent, error)
	// Stop stops the file watcher
	Stop() error
}

// FileChangeEvent represents a file change event
type FileChangeEvent struct {
	Path      string
	Type      ChangeType
	Timestamp time.Time
}

// ChangeType represents the type of file change
type ChangeType int

const (
	// Modified indicates the file was modified
	Modified ChangeType = iota
	// Created indicates the file was created
	Created
	// Deleted indicates the file was deleted
	Deleted
	// Renamed indicates the file was renamed
	Renamed
)

// String returns the string representation of ChangeType
func (c ChangeType) String() string {
	switch c {
	case Modified:
		return "modified"
	case Created:
		return "created"
	case Deleted:
		return "deleted"
	case Renamed:
		return "renamed"
	default:
		return "unknown"
	}
}
