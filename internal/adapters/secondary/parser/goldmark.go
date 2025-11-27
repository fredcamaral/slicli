package parser

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"gopkg.in/yaml.v3"

	"github.com/fredcamaral/slicli/internal/domain/ports"
)

// GoldmarkParser implements the MarkdownParser interface using Goldmark
type GoldmarkParser struct {
	md goldmark.Markdown
}

// NewGoldmarkParser creates a new Goldmark-based markdown parser
// using the shared factory for consistent configuration.
// SECURITY: Uses secure defaults (no raw HTML rendering).
func NewGoldmarkParser() *GoldmarkParser {
	return &GoldmarkParser{md: NewConfiguredGoldmark()}
}

// NewGoldmarkParserWithConfig creates a new Goldmark-based markdown parser
// with custom configuration.
func NewGoldmarkParserWithConfig(cfg GoldmarkConfig) *GoldmarkParser {
	return &GoldmarkParser{md: NewGoldmarkWithConfig(cfg)}
}

// Parse parses markdown content into structured presentation data
func (p *GoldmarkParser) Parse(ctx context.Context, content []byte) (*ports.ParsedContent, error) {
	// Extract frontmatter
	frontmatter, remaining := extractFrontmatter(content)

	// Split into slides
	slides := splitSlides(remaining)

	// Parse each slide
	parsedSlides := make([]ports.RawSlide, 0, len(slides))
	for i, slideContent := range slides {
		slide, err := p.parseSlide(ctx, slideContent, i)
		if err != nil {
			return nil, fmt.Errorf("parsing slide %d: %w", i, err)
		}
		parsedSlides = append(parsedSlides, slide)
	}

	return &ports.ParsedContent{
		Frontmatter: frontmatter,
		Slides:      parsedSlides,
	}, nil
}

// parseSlide parses a single slide's content
func (p *GoldmarkParser) parseSlide(ctx context.Context, content []byte, index int) (ports.RawSlide, error) {
	// Extract speaker notes
	contentStr := string(content)
	contentLines := strings.Split(contentStr, "\n")

	var mainContent []string
	var notes []string

	for _, line := range contentLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Note:") {
			// Extract note content after "Note:"
			noteContent := strings.TrimPrefix(trimmed, "Note:")
			notes = append(notes, strings.TrimSpace(noteContent))
		} else {
			mainContent = append(mainContent, line)
		}
	}

	return ports.RawSlide{
		Content: strings.Join(mainContent, "\n"),
		Notes:   strings.Join(notes, "\n"),
		Index:   index,
	}, nil
}

// extractFrontmatter extracts YAML frontmatter from markdown content
func extractFrontmatter(content []byte) (map[string]interface{}, []byte) {
	// Check if content starts with frontmatter delimiter
	if !bytes.HasPrefix(content, []byte("---\n")) && !bytes.HasPrefix(content, []byte("---\r\n")) {
		return nil, content
	}

	// Find the end of frontmatter
	lines := bytes.Split(content, []byte("\n"))
	endIndex := -1

	for i := 1; i < len(lines); i++ {
		line := bytes.TrimSpace(lines[i])
		if bytes.Equal(line, []byte("---")) {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		// No closing delimiter found
		return nil, content
	}

	// Extract frontmatter content
	frontmatterLines := lines[1:endIndex]
	frontmatterBytes := bytes.Join(frontmatterLines, []byte("\n"))

	// Parse YAML
	var frontmatter map[string]interface{}
	if len(frontmatterBytes) == 0 {
		// Empty frontmatter
		frontmatter = make(map[string]interface{})
	} else if err := yaml.Unmarshal(frontmatterBytes, &frontmatter); err != nil {
		// If parsing fails, return original content
		return nil, content
	}

	// Return frontmatter and remaining content
	remainingLines := lines[endIndex+1:]
	remaining := bytes.Join(remainingLines, []byte("\n"))

	return frontmatter, remaining
}

// splitSlides splits content into individual slides
func splitSlides(content []byte) [][]byte {
	// Split on horizontal rule (---)
	contentStr := string(content)

	// Handle different line endings
	contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")

	// Split by slide delimiter
	slideStrings := strings.Split(contentStr, "\n---\n")

	// Convert back to byte slices
	slides := make([][]byte, 0, len(slideStrings))
	for _, slide := range slideStrings {
		trimmed := strings.TrimSpace(slide)
		if trimmed != "" {
			slides = append(slides, []byte(trimmed))
		}
	}

	// If no slides found, treat entire content as one slide
	if len(slides) == 0 {
		return [][]byte{content}
	}

	return slides
}
