package ports

import (
	"context"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// RenderedSlide represents a slide after rendering
type RenderedSlide struct {
	Slide     *entities.Slide
	HTML      string
	NotesHTML string
}

// PresentationParser defines the interface for parsing markdown presentations
type PresentationParser interface {
	// Parse converts markdown content into a presentation
	Parse(content []byte) (*entities.Presentation, error)
}

// SlideRenderer defines the interface for rendering slides to HTML
type SlideRenderer interface {
	// RenderSlide converts a slide's markdown content to HTML
	RenderSlide(slide *entities.Slide) (*RenderedSlide, error)
}

// PresentationService defines the main service interface for presentations
type PresentationService interface {
	// LoadPresentation loads a presentation from a file path
	LoadPresentation(ctx context.Context, path string) (*entities.Presentation, error)

	// ParsePresentation parses markdown content into a presentation
	ParsePresentation(ctx context.Context, content []byte) (*entities.Presentation, error)

	// RenderSlides renders all slides in a presentation
	RenderSlides(ctx context.Context, presentation *entities.Presentation) ([]RenderedSlide, error)

	// WatchPresentation watches a presentation file for changes
	WatchPresentation(ctx context.Context, path string) (<-chan FileChangeEvent, error)

	// ApplyTheme applies a theme to a presentation
	ApplyTheme(ctx context.Context, presentation *entities.Presentation, themeName string) error
}

// ServerService defines the interface for serving presentations
type ServerService interface {
	// Serve starts the HTTP server for a presentation
	Serve(ctx context.Context, path string, port int, host string, openBrowser bool) error

	// Stop gracefully stops the server
	Stop(ctx context.Context) error
}

// NotesService defines the interface for managing speaker notes
type NotesService interface {
	// GetNotes retrieves speaker notes for a specific slide
	GetNotes(slideID string) (*entities.SpeakerNotes, error)

	// SetNotes sets speaker notes for a specific slide
	SetNotes(slideID string, notes *entities.SpeakerNotes) error

	// ExtractNotes extracts notes from slide content
	ExtractNotes(content string) (mainContent string, notesContent string)

	// ConvertNotesToHTML converts markdown notes to HTML
	ConvertNotesToHTML(notes string) string
}

// PresentationSync defines the interface for presentation synchronization
type PresentationSync interface {
	// Subscribe adds a client to receive sync events
	Subscribe(clientID string) <-chan entities.SyncEvent

	// Unsubscribe removes a client from sync events
	Unsubscribe(clientID string)

	// Broadcast sends an event to all connected clients
	Broadcast(event entities.SyncEvent) error

	// GetState returns the current presenter state
	GetState() *entities.PresenterState

	// Stop stops the sync service
	Stop()
}

// ExportService defines the interface for presentation export functionality
type ExportService interface {
	// Export exports a presentation to the specified format
	Export(ctx context.Context, presentation *entities.Presentation, options interface{}) (interface{}, error)

	// GetSupportedFormats returns a list of supported export formats
	GetSupportedFormats() []string

	// GetTempDir returns the temporary directory path for exports
	GetTempDir() string
}
