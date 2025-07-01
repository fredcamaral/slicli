package entities

import (
	"errors"
	"strconv"
	"strings"
)

// Slide represents a single slide in a presentation
type Slide struct {
	// ID is a unique identifier for the slide
	ID string `json:"id,omitempty"`

	// Index is the slide position in the presentation (0-based)
	Index int `json:"index"`

	// Title is extracted from the first H1 heading or generated
	Title string `json:"title"`

	// Content is the raw markdown content of the slide
	Content string `json:"content"`

	// HTML is the rendered HTML content (populated during rendering)
	HTML string `json:"html,omitempty"`

	// Notes contains speaker notes for this slide
	Notes string `json:"notes,omitempty"`

	// Metadata contains slide-specific frontmatter (if any)
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// Validate ensures the slide has valid content
func (s *Slide) Validate() error {
	if strings.TrimSpace(s.Content) == "" {
		return errors.New("slide content cannot be empty")
	}

	if s.Index < 0 {
		return errors.New("slide index must be non-negative")
	}

	return nil
}

// ExtractTitle attempts to extract the slide title from content
func (s *Slide) ExtractTitle() string {
	lines := strings.Split(s.Content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimPrefix(trimmed, "# ")
		}
	}

	// If no H1 found, generate a title
	return "Slide " + strconv.Itoa(s.Index+1)
}

// HasNotes returns true if the slide has speaker notes
func (s *Slide) HasNotes() bool {
	return strings.TrimSpace(s.Notes) != ""
}

// ContentWithoutNotes returns the slide content without speaker notes
// (notes are assumed to be lines starting with "Note:")
func (s *Slide) ContentWithoutNotes() string {
	lines := strings.Split(s.Content, "\n")
	var contentLines []string

	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "Note:") {
			contentLines = append(contentLines, line)
		}
	}

	return strings.Join(contentLines, "\n")
}
