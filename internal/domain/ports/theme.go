package ports

import (
	"context"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// ThemeLoader loads themes from the filesystem
type ThemeLoader interface {
	// Load loads a theme by name
	Load(ctx context.Context, name string) (*entities.ThemeEngine, error)

	// List returns information about all available themes
	List(ctx context.Context) ([]entities.ThemeInfo, error)

	// Exists checks if a theme exists
	Exists(ctx context.Context, name string) bool

	// Reload reloads a theme (for hot reload)
	Reload(ctx context.Context, name string) (*entities.ThemeEngine, error)
}

// ThemeCache caches loaded themes and assets
type ThemeCache interface {
	// Get retrieves a cached theme
	Get(name string) (*entities.ThemeEngine, bool)

	// Set stores a theme in the cache
	Set(name string, theme *entities.ThemeEngine)

	// Remove removes a theme from the cache
	Remove(name string)

	// Clear clears all cached themes
	Clear()

	// Stats returns cache statistics
	Stats() entities.CacheStats
}

// AssetProcessor processes theme assets (CSS, JS)
type AssetProcessor interface {
	// Process processes any asset based on content type
	Process(content []byte, contentType string, variables map[string]string) ([]byte, error)

	// ProcessCSS processes CSS with variable substitution
	ProcessCSS(content []byte, variables map[string]string) ([]byte, error)

	// ProcessJS processes JavaScript files
	ProcessJS(content []byte, variables map[string]string) ([]byte, error)

	// MinifyCSS minifies CSS content
	MinifyCSS(content []byte) ([]byte, error)

	// MinifyJS minifies JavaScript content
	MinifyJS(content []byte) ([]byte, error)
}

// ThemeService manages themes
type ThemeService interface {
	// GetTheme retrieves a theme by name
	GetTheme(ctx context.Context, name string) (*entities.ThemeEngine, error)

	// ListThemes lists all available themes
	ListThemes(ctx context.Context) ([]entities.ThemeInfo, error)

	// RenderPresentation renders a presentation with a theme
	RenderPresentation(theme *entities.ThemeEngine, presentation *entities.Presentation) ([]byte, error)

	// RenderSlide renders a single slide
	RenderSlide(theme *entities.ThemeEngine, slide *entities.Slide, slideNumber int, totalSlides int) ([]byte, error)

	// ServeAsset serves a theme asset
	ServeAsset(theme *entities.ThemeEngine, path string) (*entities.ThemeAsset, error)

	// ReloadTheme reloads a theme (for development)
	ReloadTheme(ctx context.Context, name string) error
}
