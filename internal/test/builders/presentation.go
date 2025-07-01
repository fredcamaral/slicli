package builders

import (
	"strconv"
	"time"

	"github.com/fredcamaral/slicli/internal/domain/entities"
)

// PresentationBuilder helps build Presentation entities for testing
type PresentationBuilder struct {
	presentation *entities.Presentation
}

// NewPresentationBuilder creates a new presentation builder with sensible defaults
func NewPresentationBuilder() *PresentationBuilder {
	return &PresentationBuilder{
		presentation: &entities.Presentation{
			Title:    "Test Presentation",
			Author:   "Test Author",
			Date:     time.Now(),
			Theme:    "default",
			Slides:   []entities.Slide{},
			Metadata: make(map[string]interface{}),
		},
	}
}

// WithTitle sets the presentation title
func (b *PresentationBuilder) WithTitle(title string) *PresentationBuilder {
	b.presentation.Title = title
	return b
}

// WithAuthor sets the presentation author
func (b *PresentationBuilder) WithAuthor(author string) *PresentationBuilder {
	b.presentation.Author = author
	return b
}

// WithDate sets the presentation date
func (b *PresentationBuilder) WithDate(date time.Time) *PresentationBuilder {
	b.presentation.Date = date
	return b
}

// WithTheme sets the presentation theme
func (b *PresentationBuilder) WithTheme(theme string) *PresentationBuilder {
	b.presentation.Theme = theme
	return b
}

// WithSlides sets the presentation slides
func (b *PresentationBuilder) WithSlides(slides []entities.Slide) *PresentationBuilder {
	b.presentation.Slides = slides
	return b
}

// WithSlide adds a single slide to the presentation
func (b *PresentationBuilder) WithSlide(slide entities.Slide) *PresentationBuilder {
	b.presentation.Slides = append(b.presentation.Slides, slide)
	return b
}

// WithSlideCount adds the specified number of default slides
func (b *PresentationBuilder) WithSlideCount(count int) *PresentationBuilder {
	for i := 0; i < count; i++ {
		slide := NewSlideBuilder().
			WithID(i + 1).
			WithTitle("Slide " + string(rune(i+1+'0'))).
			Build()
		b.presentation.Slides = append(b.presentation.Slides, slide)
	}
	return b
}

// WithMetadata sets custom metadata
func (b *PresentationBuilder) WithMetadata(key string, value interface{}) *PresentationBuilder {
	if b.presentation.Metadata == nil {
		b.presentation.Metadata = make(map[string]interface{})
	}
	b.presentation.Metadata[key] = value
	return b
}

// Build creates the final Presentation entity
func (b *PresentationBuilder) Build() *entities.Presentation {
	// Deep copy to prevent mutation
	return &entities.Presentation{
		Title:    b.presentation.Title,
		Author:   b.presentation.Author,
		Date:     b.presentation.Date,
		Theme:    b.presentation.Theme,
		Slides:   append([]entities.Slide{}, b.presentation.Slides...),
		Metadata: copyMetadata(b.presentation.Metadata),
	}
}

// SlideBuilder helps build Slide entities for testing
type SlideBuilder struct {
	slide *entities.Slide
}

// NewSlideBuilder creates a new slide builder with sensible defaults
func NewSlideBuilder() *SlideBuilder {
	return &SlideBuilder{
		slide: &entities.Slide{
			ID:       "slide-1",
			Index:    0,
			Title:    "Test Slide",
			Content:  "# Test Slide\n\nTest content",
			HTML:     "<h1>Test Slide</h1>",
			Notes:    "Test notes",
			Metadata: make(map[string]interface{}),
		},
	}
}

// WithID sets the slide ID
func (b *SlideBuilder) WithID(id int) *SlideBuilder {
	b.slide.ID = "slide-" + strconv.Itoa(id)
	b.slide.Index = id - 1 // Convert to 0-based index
	return b
}

// WithTitle sets the slide title and content
func (b *SlideBuilder) WithTitle(title string) *SlideBuilder {
	b.slide.Title = title
	b.slide.Content = "# " + title + "\n\nTest content"
	b.slide.HTML = "<h1>" + title + "</h1>"
	return b
}

// WithHTML sets the slide HTML content
func (b *SlideBuilder) WithHTML(html string) *SlideBuilder {
	b.slide.HTML = html
	return b
}

// WithNotes sets the slide speaker notes
func (b *SlideBuilder) WithNotes(notes string) *SlideBuilder {
	b.slide.Notes = notes
	return b
}

// WithMetadata sets custom metadata
func (b *SlideBuilder) WithMetadata(key string, value interface{}) *SlideBuilder {
	if b.slide.Metadata == nil {
		b.slide.Metadata = make(map[string]interface{})
	}
	b.slide.Metadata[key] = value
	return b
}

// Build creates the final Slide entity
func (b *SlideBuilder) Build() entities.Slide {
	return entities.Slide{
		ID:       b.slide.ID,
		Index:    b.slide.Index,
		Title:    b.slide.Title,
		Content:  b.slide.Content,
		HTML:     b.slide.HTML,
		Notes:    b.slide.Notes,
		Metadata: copyMetadata(b.slide.Metadata),
	}
}

// copyMetadata creates a deep copy of metadata map
func copyMetadata(original map[string]interface{}) map[string]interface{} {
	if original == nil {
		return nil
	}
	copy := make(map[string]interface{})
	for k, v := range original {
		copy[k] = v
	}
	return copy
}

// Common presentation types for testing

// MinimalPresentation creates a minimal presentation for basic tests
func MinimalPresentation() *entities.Presentation {
	return NewPresentationBuilder().
		WithTitle("Minimal").
		WithSlideCount(1).
		Build()
}

// LargePresentation creates a presentation with many slides for performance tests
func LargePresentation() *entities.Presentation {
	return NewPresentationBuilder().
		WithTitle("Large Presentation").
		WithSlideCount(50).
		Build()
}

// PresentationWithMetadata creates a presentation with rich metadata for metadata tests
func PresentationWithMetadata() *entities.Presentation {
	return NewPresentationBuilder().
		WithTitle("Rich Metadata").
		WithMetadata("category", "technical").
		WithMetadata("duration", 30).
		WithMetadata("tags", []string{"go", "testing", "architecture"}).
		WithSlideCount(5).
		Build()
}
