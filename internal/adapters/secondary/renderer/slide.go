package renderer

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// SlideRendererAdapter adapts Goldmark to the SlideRenderer interface
type SlideRendererAdapter struct {
	md goldmark.Markdown
}

// NewSlideRendererAdapter creates a new slide renderer
func NewSlideRendererAdapter() *SlideRendererAdapter {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	return &SlideRendererAdapter{
		md: md,
	}
}

// RenderSlide converts a slide's markdown content to HTML
func (r *SlideRendererAdapter) RenderSlide(slide *entities.Slide) (*ports.RenderedSlide, error) {
	if slide == nil {
		return nil, errors.New("slide cannot be nil")
	}

	// If HTML is already rendered, use it
	if slide.HTML != "" {
		return &ports.RenderedSlide{
			Slide:     slide,
			HTML:      slide.HTML,
			NotesHTML: r.renderNotes(slide.Notes),
		}, nil
	}

	// Render markdown content to HTML
	var buf bytes.Buffer
	content := []byte(slide.Content)
	if err := r.md.Convert(content, &buf); err != nil {
		return nil, fmt.Errorf("rendering markdown: %w", err)
	}

	// Render notes if present
	notesHTML := r.renderNotes(slide.Notes)

	return &ports.RenderedSlide{
		Slide:     slide,
		HTML:      buf.String(),
		NotesHTML: notesHTML,
	}, nil
}

// renderNotes converts speaker notes to HTML
func (r *SlideRendererAdapter) renderNotes(notes string) string {
	if notes == "" {
		return ""
	}

	// Simple rendering - wrap in paragraphs
	// In the future, we might want to support markdown in notes too
	return fmt.Sprintf("<p>%s</p>", notes)
}
