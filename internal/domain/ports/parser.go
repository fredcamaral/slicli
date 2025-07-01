package ports

import (
	"context"
)

// MarkdownParser defines the interface for parsing markdown content
type MarkdownParser interface {
	Parse(ctx context.Context, content []byte) (*ParsedContent, error)
}

// ParsedContent represents the result of parsing a markdown file
type ParsedContent struct {
	Frontmatter map[string]interface{}
	Slides      []RawSlide
}

// RawSlide represents a single slide before rendering
type RawSlide struct {
	Content string
	Notes   string
	Index   int
}
