package renderer

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/yuin/goldmark"

	"github.com/fredcamaral/slicli/internal/adapters/secondary/parser"
	"github.com/fredcamaral/slicli/internal/adapters/secondary/sanitizer"
	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// SlideRendererAdapter adapts Goldmark to the SlideRenderer interface
type SlideRendererAdapter struct {
	md goldmark.Markdown
}

// NewSlideRendererAdapter creates a new slide renderer
// using the shared factory for consistent configuration.
// SECURITY: Uses secure defaults (no raw HTML rendering).
func NewSlideRendererAdapter() *SlideRendererAdapter {
	return &SlideRendererAdapter{
		md: parser.NewConfiguredGoldmark(),
	}
}

// NewSlideRendererAdapterWithConfig creates a new slide renderer
// with custom Goldmark configuration.
func NewSlideRendererAdapterWithConfig(cfg parser.GoldmarkConfig) *SlideRendererAdapter {
	return &SlideRendererAdapter{
		md: parser.NewGoldmarkWithConfig(cfg),
	}
}

// RenderSlide converts a slide's markdown content to HTML
func (r *SlideRendererAdapter) RenderSlide(slide *entities.Slide) (*ports.RenderedSlide, error) {
	if slide == nil {
		return nil, errors.New("slide cannot be nil")
	}

	// If HTML is already rendered, sanitize and use it
	if slide.HTML != "" {
		// SECURITY: Sanitize pre-rendered HTML to prevent XSS
		sanitizedHTML := sanitizer.SanitizeHTML(slide.HTML)
		return &ports.RenderedSlide{
			Slide:     slide,
			HTML:      sanitizedHTML,
			NotesHTML: r.renderNotes(slide.Notes),
		}, nil
	}

	// Render markdown content to HTML
	var buf bytes.Buffer
	content := []byte(slide.Content)
	if err := r.md.Convert(content, &buf); err != nil {
		return nil, fmt.Errorf("rendering markdown: %w", err)
	}

	// SECURITY: Sanitize rendered HTML to prevent XSS
	sanitizedHTML := sanitizer.SanitizeHTML(buf.String())

	// Render notes if present
	notesHTML := r.renderNotes(slide.Notes)

	return &ports.RenderedSlide{
		Slide:     slide,
		HTML:      sanitizedHTML,
		NotesHTML: notesHTML,
	}, nil
}

// renderNotes converts speaker notes to HTML
func (r *SlideRendererAdapter) renderNotes(notes string) string {
	if notes == "" {
		return ""
	}

	// Simple rendering - wrap in paragraphs
	// SECURITY: Sanitize notes content to prevent XSS
	sanitizedNotes := sanitizer.SanitizeHTML(notes)
	return fmt.Sprintf("<p>%s</p>", sanitizedNotes)
}
