package entities

import (
	"errors"
	"fmt"
	"time"
)

// Presentation represents a complete slide presentation with metadata and slides
type Presentation struct {
	// ID is a unique identifier for the presentation
	ID string `yaml:"-" json:"id,omitempty"`

	// Title is the presentation title
	Title string `yaml:"title" json:"title"`

	// Theme specifies the visual theme to use
	Theme string `yaml:"theme" json:"theme"`

	// Author is the presentation creator
	Author string `yaml:"author" json:"author"`

	// Date is when the presentation was created/updated
	Date time.Time `yaml:"date" json:"date"`

	// Metadata contains any additional frontmatter fields
	Metadata map[string]interface{} `yaml:",inline" json:"metadata,omitempty"`

	// Slides contains all presentation slides in order
	Slides []Slide `yaml:"-" json:"slides"`
}

// Validate ensures the presentation has valid required fields
func (p *Presentation) Validate() error {
	if p.Title == "" {
		return errors.New("presentation title is required")
	}

	if len(p.Slides) == 0 {
		return errors.New("presentation must have at least one slide")
	}

	// Validate each slide
	for i, slide := range p.Slides {
		if err := slide.Validate(); err != nil {
			return fmt.Errorf("slide %d validation failed: %w", i+1, err)
		}
	}

	// Set default theme if not specified
	if p.Theme == "" {
		p.Theme = "default"
	}

	return nil
}

// GetSlideByIndex returns a slide by its index (0-based)
func (p *Presentation) GetSlideByIndex(index int) (*Slide, error) {
	if index < 0 || index >= len(p.Slides) {
		return nil, fmt.Errorf("slide index %d out of range (0-%d)", index, len(p.Slides)-1)
	}
	return &p.Slides[index], nil
}

// SlideCount returns the total number of slides
func (p *Presentation) SlideCount() int {
	return len(p.Slides)
}
