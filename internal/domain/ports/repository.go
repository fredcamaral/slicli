package ports

import (
	"context"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// RepositoryChangeEvent represents a file system change event for repositories
type RepositoryChangeEvent struct {
	Path      string
	Operation string // "create", "update", "delete"
	Error     error
}

// PresentationRepository defines the interface for loading and watching presentations
type PresentationRepository interface {
	// Load reads and returns a presentation from the given path
	Load(ctx context.Context, path string) (*entities.Presentation, error)

	// Watch monitors a presentation file for changes and sends events
	Watch(ctx context.Context, path string) (<-chan RepositoryChangeEvent, error)
}

// ThemeRepository defines the interface for managing themes
type ThemeRepository interface {
	// Get retrieves a theme by name
	Get(ctx context.Context, name string) (*entities.Theme, error)

	// List returns all available themes
	List(ctx context.Context) ([]*entities.Theme, error)

	// Load loads a custom theme from a directory
	Load(ctx context.Context, path string) (*entities.Theme, error)
}
