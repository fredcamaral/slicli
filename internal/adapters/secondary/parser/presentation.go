package parser

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"github.com/fredcamaral/slicli/internal/domain/entities"
	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// PresentationParserAdapter adapts the MarkdownParser to the PresentationParser interface
type PresentationParserAdapter struct {
	markdownParser ports.MarkdownParser
	goldmark       goldmark.Markdown
}

// NewPresentationParserAdapter creates a new presentation parser adapter
func NewPresentationParserAdapter(markdownParser ports.MarkdownParser) *PresentationParserAdapter {
	// Create a Goldmark instance for rendering HTML
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Typographer,
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	return &PresentationParserAdapter{
		markdownParser: markdownParser,
		goldmark:       md,
	}
}

// Parse implements the PresentationParser interface
func (p *PresentationParserAdapter) Parse(content []byte) (*entities.Presentation, error) {
	// Parse markdown content
	parsed, err := p.markdownParser.Parse(context.Background(), content)
	if err != nil {
		return nil, fmt.Errorf("parsing markdown: %w", err)
	}

	// Create presentation from parsed content
	presentation := &entities.Presentation{
		Metadata: parsed.Frontmatter,
		Slides:   make([]entities.Slide, 0, len(parsed.Slides)),
	}

	// Extract metadata
	if title, ok := getStringFromMap(parsed.Frontmatter, "title"); ok {
		presentation.Title = title
	}
	if author, ok := getStringFromMap(parsed.Frontmatter, "author"); ok {
		presentation.Author = author
	}
	if theme, ok := getStringFromMap(parsed.Frontmatter, "theme"); ok {
		presentation.Theme = theme
	}
	if dateStr, ok := getStringFromMap(parsed.Frontmatter, "date"); ok {
		if date, err := time.Parse("2006-01-02", dateStr); err == nil {
			presentation.Date = date
		}
	}

	// If no date is set, use current date
	if presentation.Date.IsZero() {
		presentation.Date = time.Now()
	}

	// Convert raw slides to domain entities
	for _, rawSlide := range parsed.Slides {
		slide := entities.Slide{
			Index:   rawSlide.Index,
			Content: rawSlide.Content,
			Notes:   rawSlide.Notes,
		}

		// Extract title from content
		slide.Title = slide.ExtractTitle()

		// Render HTML content
		htmlContent, err := p.renderMarkdown(rawSlide.Content)
		if err != nil {
			return nil, fmt.Errorf("rendering slide %d: %w", rawSlide.Index, err)
		}
		slide.HTML = htmlContent

		presentation.Slides = append(presentation.Slides, slide)
	}

	// Validate the presentation
	if err := presentation.Validate(); err != nil {
		return nil, fmt.Errorf("invalid presentation: %w", err)
	}

	return presentation, nil
}

// renderMarkdown renders markdown content to HTML
func (p *PresentationParserAdapter) renderMarkdown(content string) (string, error) {
	var buf bytes.Buffer
	if err := p.goldmark.Convert([]byte(content), &buf); err != nil {
		return "", fmt.Errorf("rendering markdown: %w", err)
	}
	return buf.String(), nil
}

// getStringFromMap safely extracts a string value from a map
func getStringFromMap(m map[string]interface{}, key string) (string, bool) {
	if m == nil {
		return "", false
	}

	val, exists := m[key]
	if !exists {
		return "", false
	}

	str, ok := val.(string)
	return str, ok
}
